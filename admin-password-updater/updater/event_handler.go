package updater

import (
	"fmt"
	"net/http"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"gopkg.in/ini.v1"
)

type PasswordUpdater struct {
	DefaultUserFile string
	AdminFile       string
	Watcher         *fsnotify.Watcher
	WatchDir        string
	Done            chan<- bool
	Log             logr.Logger
	Rmqc            RabbitClient
}

type RabbitClient interface {
	// RabbitMQ Management API functions
	GetUser(string) (*rabbithole.UserInfo, error)
	PutUser(string, rabbithole.UserSettings) (*http.Response, error)
	Whoami() (*rabbithole.WhoamiInfo, error)
	// Field getters and setters
	GetUsername() string
	SetUsername(string)
	SetPassword(string)
}

func (u *PasswordUpdater) HandleEvents() {
	for {
		select {
		case event, ok := <-u.Watcher.Events:
			if !ok {
				u.Log.V(0).Info("watcher events channel is closed, exiting...", "directory", u.WatchDir)
				u.Done <- true
				return
			}
			if fileChanged(u.DefaultUserFile, event) {
				u.Log.V(2).Info("file system event", "file", u.DefaultUserFile, "operation", event.Op.String())

				// read default user username and (new) password
				defaultUserCfg, err := ini.Load(u.DefaultUserFile)
				if err != nil {
					u.Log.Error(err, "failed to load INI data source", "file", u.DefaultUserFile)
					u.Done <- true
					return
				}
				defaultUser := defaultUserCfg.Section("").Key("default_user").String()
				newPasswd := defaultUserCfg.Section("").Key("default_pass").String()

				// read admin username and (old) password
				adminCfg, err := ini.Load(u.AdminFile)
				if err != nil {
					u.Log.Error(err, "failed to load INI data source", "file", u.AdminFile)
					u.Done <- true
					return
				}
				adminSection := adminCfg.Section("default")
				adminUser := adminSection.Key("username").String()
				oldPasswd := adminSection.Key("password").String()

				if defaultUser != adminUser {
					u.Log.V(0).Info("exiting because usernames do not match",
						"default-user", defaultUser, "default-user-file", u.DefaultUserFile,
						"admin-user", adminUser, "admin-file", u.AdminFile)
					u.Done <- true
					return
				}
				if newPasswd == oldPasswd {
					u.Log.V(2).Info("passwords already match, nothing to do", "username", defaultUser)
					break
				}

				u.Rmqc.SetUsername(adminUser)
				u.Rmqc.SetPassword(oldPasswd)

				if err := u.updateInRabbitMQ(adminUser, newPasswd); err != nil {
					break
				}

				u.Log.V(4).Info("copying new password...", "source", u.DefaultUserFile, "target", u.AdminFile)
				adminSection.Key("password").SetValue(newPasswd)
				if err := adminCfg.SaveTo(u.AdminFile); err != nil {
					u.Log.Error(err, "failed to write new password", "file", u.AdminFile)
					u.Done <- true
					return
				}
				u.Log.V(2).Info("copied new password", "source", u.DefaultUserFile, "target", u.AdminFile)
			} else {
				u.Log.V(4).Info("file system event", "file", event.Name, "operation", event.Op.String())
			}

		case err, ok := <-u.Watcher.Errors:
			if !ok {
				u.Log.V(0).Info("watcher errors channel is closed, exiting...")
				u.Done <- true
				return
			}
			u.Log.Error(err, "failed to watch", "directory", u.WatchDir)
		}
	}

}

func fileChanged(defaultUserFile string, event fsnotify.Event) bool {
	return event.Name == defaultUserFile &&
		(event.Op&fsnotify.Create == fsnotify.Create ||
			event.Op&fsnotify.Write == fsnotify.Write)
}

// updateInRabbitMQ sets newPasswd for existingUser in the RabbitMQ server.
// It returns an error if password cannot be updated.
func (u *PasswordUpdater) updateInRabbitMQ(existingUser, newPasswd string) error {
	pathUsers := "/api/users/" + existingUser

	user, err := u.Rmqc.GetUser(existingUser)
	if err != nil {
		return u.handleHTTPError(err, http.MethodGet, pathUsers, newPasswd)
	}

	// We succeeded to fetch user tags, continue to update user password.
	newUserSettings := rabbithole.UserSettings{
		Name:             existingUser,
		Tags:             user.Tags,
		Password:         newPasswd,
		HashingAlgorithm: user.HashingAlgorithm,
	}
	resp, err := u.Rmqc.PutUser(existingUser, newUserSettings)
	if err != nil {
		return u.handleHTTPError(err, http.MethodPut, pathUsers, newPasswd)
	}

	u.Log.V(3).Info("HTTP response", "method", http.MethodPut, "path", pathUsers, "status", resp.Status)
	u.Log.V(2).Info("updated password on RabbitMQ server", "user", existingUser)
	return nil
}

func (u *PasswordUpdater) handleHTTPError(err error, httpMethod, pathUsers, newPasswd string) error {
	// as returned in
	// https://github.com/michaelklishin/rabbit-hole/blob/1de83b96b8ba1e29afd003143a9d8a8234d4e913/client.go#L153
	if err.Error() == "Error: API responded with a 401 Unauthorized" {
		// Only one node in a multi node RabbitMQ cluster will update the password.
		// All other nodes are expected to run into this branch.
		u.Log.V(2).Info(
			"HTTP request with old password returned 401 Unauthorized, therefore trying to authenticate with new password...",
			"method", httpMethod, "path", pathUsers)
		u.Rmqc.SetPassword(newPasswd)
		return u.authenticate()
	}
	u.Log.Error(err, "HTTP request failed", "method", httpMethod, "path", pathUsers)
	return err
}

// authenticate checks whether authentication succeeds.
// It queries /api/whoami (although it could query any other endpoint requiring basic auth).
// Returns an error if authentication fails.
func (u *PasswordUpdater) authenticate() error {
	const pathWhoAmI = "/api/whoami"
	_, err := u.Rmqc.Whoami()
	if err != nil {
		u.Log.Error(err, fmt.Sprintf("failed to GET %s with new password", pathWhoAmI))
		return err
	}
	u.Log.V(2).Info(fmt.Sprintf(
		"GET %s with new password succeeded, therefore skipping PUT %s...", pathWhoAmI, "/api/users/"+u.Rmqc.GetUsername()))
	return nil
}

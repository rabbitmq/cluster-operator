package updater_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/rabbitmq/cluster-operator/admin-password-updater/updater"
	"gopkg.in/ini.v1"
	"k8s.io/klog/v2/klogr"
)

const (
	defaultFileSection     = ""
	defaultFileUserKey     = "default_user"
	defaultFilePasswordKey = "default_pass"
	adminFileSection       = "default"
	adminFileUserKey       = "username"
	adminFilePasswordKey   = "password"
)

var (
	defaultUserFile = filepath.Join("test", "default-user.conf")
	adminFile       = filepath.Join("test", "rabbitmqadmin.conf")
)

var _ = Describe("EventHandler", func() {
	const watchDir = "test"
	var (
		u          *PasswordUpdater
		fakeClient *fakeRabbitClient
		done       chan bool
		// as returned in https://github.com/michaelklishin/rabbit-hole/blob/1de83b96b8ba1e29afd003143a9d8a8234d4e913/client.go#L153
		errUnauthorized = errors.New("Error: API responded with a 401 Unauthorized")
	)

	BeforeEach(func() {
		// Remove trailing new line
		ini.PrettySection = false
		initConfigFiles()
		log := klogr.New()
		fakeClient = &fakeRabbitClient{}
		watcher, err := fsnotify.NewWatcher()
		Expect(err).ToNot(HaveOccurred())
		Expect(watcher.Add(watchDir)).To(Succeed())
		done = make(chan bool, 1)
		u = &PasswordUpdater{
			DefaultUserFile: defaultUserFile,
			AdminFile:       adminFile,
			Watcher:         watcher,
			WatchDir:        watchDir,
			Done:            done,
			Log:             log,
			Rmqc:            fakeClient,
		}
		go u.HandleEvents()
	})

	AfterEach(func() {
		u.Watcher.Close()
		initConfigFiles()
	})

	When("default user file cannot be parsed", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(defaultUserFile, []byte("invalid INI"), 0644)).To(Succeed())
		})
		It("exits", func() {
			Eventually(done).Should(Receive())
		})
	})
	When("admin file cannot be parsed", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(adminFile, []byte("invalid INI"), 0644)).To(Succeed())
			// trigger file event
			write(defaultUserFile, defaultFileSection, defaultFilePasswordKey, "pwd1")
		})
		It("exits", func() {
			Eventually(done).Should(Receive())
		})
	})

	When("passwords in files already match", func() {
		BeforeEach(func() {
			// trigger file event with same password
			write(defaultUserFile, defaultFileSection, defaultFilePasswordKey, "pwd1")
		})
		It("does not talk to RabbitMQ", func() {
			Consistently(func() string {
				return fakeClient.getUserArg
			}).Should(BeEmpty())

			Consistently(func() putUserArg {
				return fakeClient.putUserArg
			}).Should(Equal(putUserArg{}))
		})
	})

	When("usernames in files do not match", func() {
		BeforeEach(func() {
			write(defaultUserFile, defaultFileSection, defaultFileUserKey, "otherUser")
		})
		It("exits because this tool only updates the password", func() {
			Eventually(done).Should(Receive())
		})
	})

	When("password in default user file updates", func() {
		JustBeforeEach(func() {
			write(defaultUserFile, defaultFileSection, defaultFilePasswordKey, "pwd2")
		})
		When("password in RabbitMQ is not yet up-to-date", func() {
			BeforeEach(func() {
				fakeClient.getUserReturn = getUserReturn{
					userInfo: &rabbithole.UserInfo{
						HashingAlgorithm: "myalgo",
						Tags:             rabbithole.UserTags{"mytag"},
					}}
				fakeClient.putUserReturn = putUserReturn{
					resp: &http.Response{
						Status: "204 No Content",
					}}
			})
			It("updates password in RabbitMQ", func() {
				Eventually(func() string {
					return fakeClient.getUserArg
				}).Should(Equal("myuser"))
				expectedUserSettings := rabbithole.UserSettings{
					Name:             "myuser",
					Tags:             rabbithole.UserTags{"mytag"},
					Password:         "pwd2",
					HashingAlgorithm: "myalgo",
				}
				Expect(fakeClient.putUserArg).To(Equal(putUserArg{"myuser", expectedUserSettings}))
			})
			It("copies new password to admin conf", func() {
				Eventually(func() string {
					return read(adminFile, adminFileSection, adminFilePasswordKey)
				}).Should(Equal("pwd2"))
			})
		})
		When("password in RabbitMQ is already up-to-date", func() {
			BeforeEach(func() {
				fakeClient.whoamiReturn = whoamiReturn{err: nil}
			})
			Context("before GET /api/users/myuser", func() {
				BeforeEach(func() {
					fakeClient.getUserReturn = getUserReturn{
						// as returned in https://github.com/michaelklishin/rabbit-hole/blob/1de83b96b8ba1e29afd003143a9d8a8234d4e913/client.go#L153
						err: errUnauthorized}
				})
				It("does not PUT /api/users/myuser", func() {
					Consistently(func() putUserArg {
						return fakeClient.putUserArg
					}).Should(Equal(putUserArg{}))
				})
				It("copies new password to admin conf", func() {
					Eventually(func() string {
						return read(adminFile, adminFileSection, adminFilePasswordKey)
					}).Should(Equal("pwd2"))
				})
			})
			Context("after GET /api/users/myuser", func() {
				BeforeEach(func() {
					fakeClient.getUserReturn = getUserReturn{
						userInfo: &rabbithole.UserInfo{
							HashingAlgorithm: "myalgo",
							Tags:             rabbithole.UserTags{"mytag"},
						}}
					fakeClient.putUserReturn = putUserReturn{
						err: errUnauthorized}
				})
				It("copies new password to admin conf", func() {
					Eventually(func() string {
						return read(adminFile, adminFileSection, adminFilePasswordKey)
					}).Should(Equal("pwd2"))
				})
			})
		})
		When("neither old nor new password is valid", func() {
			BeforeEach(func() {
				fakeClient.getUserReturn = getUserReturn{err: errUnauthorized}
				fakeClient.whoamiReturn = whoamiReturn{err: errors.New("cannot authenticate with new password either")}
			})
			It("does not copy new password to admin conf", func() {
				Consistently(func() string {
					return read(adminFile, adminFileSection, adminFilePasswordKey)
				}).Should(Equal("pwd1"))
			})
		})
	})
})

func initConfigFiles() {
	cfg := ini.Empty()
	_, err := cfg.Section(defaultFileSection).NewKey(defaultFileUserKey, "myuser")
	Expect(err).ToNot(HaveOccurred())
	_, err = cfg.Section(defaultFileSection).NewKey(defaultFilePasswordKey, "pwd1")
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg.SaveTo(defaultUserFile)).To(Succeed())

	cfg = ini.Empty()
	section, err := cfg.NewSection(adminFileSection)
	Expect(err).ToNot(HaveOccurred())
	_, err = section.NewKey(adminFileUserKey, "myuser")
	Expect(err).ToNot(HaveOccurred())
	_, err = section.NewKey(adminFilePasswordKey, "pwd1")
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg.SaveTo(adminFile)).To(Succeed())
}

func read(file, section, key string) string {
	cfg, err := ini.Load(file)
	Expect(err).ToNot(HaveOccurred())
	return cfg.Section(section).Key(key).String()
}

func write(file, section, key, value string) {
	cfg, err := ini.Load(file)
	Expect(err).ToNot(HaveOccurred())
	cfg.Section(section).Key(key).SetValue(value)
	Expect(cfg.SaveTo(file)).To(Succeed())
}

type fakeRabbitClient struct {
	Username string
	Password string
	// Method arguments.
	// Enables us to assert after the test ran.
	getUserArg string
	putUserArg putUserArg
	// Method return values.
	// Enables us to stub before the test runs.
	getUserReturn getUserReturn
	putUserReturn putUserReturn
	whoamiReturn  whoamiReturn
}
type getUserReturn struct {
	userInfo *rabbithole.UserInfo
	err      error
}
type putUserArg struct {
	username string
	info     rabbithole.UserSettings
}
type putUserReturn struct {
	resp *http.Response
	err  error
}
type whoamiReturn struct {
	info *rabbithole.WhoamiInfo
	err  error
}

func (frc *fakeRabbitClient) GetUser(username string) (*rabbithole.UserInfo, error) {
	frc.getUserArg = username
	r := frc.getUserReturn
	return r.userInfo, r.err
}
func (frc *fakeRabbitClient) PutUser(username string, info rabbithole.UserSettings) (*http.Response, error) {
	frc.putUserArg = putUserArg{
		username: username,
		info:     info,
	}
	r := frc.putUserReturn
	return r.resp, r.err
}
func (frc *fakeRabbitClient) Whoami() (*rabbithole.WhoamiInfo, error) {
	r := frc.whoamiReturn
	return r.info, r.err
}
func (frc *fakeRabbitClient) GetUsername() string {
	return frc.Username
}
func (frc *fakeRabbitClient) SetUsername(username string) {
	frc.Username = username
}
func (frc *fakeRabbitClient) SetPassword(passwd string) {
	frc.Password = passwd
}

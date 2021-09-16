package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fsnotify/fsnotify"
	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/cluster-operator/admin-password-updater/updater"
	"gopkg.in/ini.v1"
)

func main() {
	var managementURI, caFile string
	u := &updater.PasswordUpdater{}

	flag.StringVar(
		&u.DefaultUserFile,
		"default-user-file",
		"/etc/rabbitmq/conf.d/11-default_user.conf",
		"Absolute path to file containing default user username and (updated) password. "+
			"Its directory will be watched for changes.")
	flag.StringVar(
		&u.AdminFile,
		"admin-file",
		"/var/lib/rabbitmq/.rabbitmqadmin.conf",
		"Absolute path to file used by rabbitmqadmin CLI. "+
			"It contains RabbitMQ admin username (must be the same as default user username) and (old) password.")
	flag.StringVar(
		&managementURI,
		"management-uri",
		"http://127.0.0.1:15672",
		"RabbitMQ Management URI")
	flag.StringVar(
		&caFile,
		"ca-file",
		"/etc/rabbitmq-tls/ca.crt",
		"This file contains the trusted certificate for RabbitMQ server authentication.")
	klog.InitFlags(nil)
	flag.Parse()
	log := klogr.New().WithName("password-updater")
	u.Log = log

	rabbitClient, err := newRabbitClient(log, managementURI, caFile)
	if err != nil {
		log.Error(err, "failed to create RabbitMQ client")
		return
	}
	u.Rmqc = rabbitClient

	// Watch the directory because the file gets re-created (i.e. first removed and then created)
	// when password is updated by Vault agent and fsnotify will stop watching a re-created file.
	watchDir := filepath.Dir(u.DefaultUserFile)
	u.WatchDir = watchDir

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err, "failed to create watcher")
		return
	}
	defer watcher.Close()
	u.Watcher = watcher

	// Remove trailing new line (.rabbitmqadmin.conf has only one section).
	ini.PrettySection = false

	// This channel will contain a value when the Pod gets terminated.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	// This channel will contain a value when our program terminates itself.
	// This is preferred over calling os.Exit() because os.Exit() does not run deferred functions.
	done := make(chan bool, 1)
	u.Done = done

	go u.HandleEvents()

	log.V(1).Info("start watching", "directory", watchDir)
	if err := watcher.Add(watchDir); err != nil {
		log.Error(err, "cannot watch", "directory", watchDir)
		return
	}

	select {
	case sig := <-sigs:
		log.V(1).Info("terminating", "signal", sig.String())
	case <-done:
		log.V(1).Info("terminating")
	}
}

func newRabbitClient(log logr.Logger, managementURI, caFile string) (updater.RabbitClient, error) {
	if strings.HasPrefix(managementURI, "https") {
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			log.Error(err, "failed to read CA file", "file", caFile)
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig := &tls.Config{
			RootCAs: caCertPool,
		}
		transport := &http.Transport{TLSClientConfig: tlsConfig}
		rmqc, err := rabbithole.NewTLSClient(managementURI, "", "", transport)
		if err != nil {
			log.Error(err, "failed to create rabbithole TLS client", "uri", managementURI, "ca-file", caFile)
			return nil, err
		}
		return rabbitHoleClientWrapper{rmqc}, nil
	}
	rmqc, err := rabbithole.NewClient(managementURI, "", "")
	if err != nil {
		log.Error(err, "failed to create rabbithole client", "uri", managementURI)
		return nil, err
	}
	return rabbitHoleClientWrapper{rmqc}, nil
}

type rabbitHoleClientWrapper struct {
	rabbitHoleClient *rabbithole.Client
}

func (w rabbitHoleClientWrapper) GetUser(username string) (*rabbithole.UserInfo, error) {
	return w.rabbitHoleClient.GetUser(username)
}
func (w rabbitHoleClientWrapper) PutUser(username string, info rabbithole.UserSettings) (*http.Response, error) {
	return w.rabbitHoleClient.PutUser(username, info)
}
func (w rabbitHoleClientWrapper) Whoami() (*rabbithole.WhoamiInfo, error) {
	return w.rabbitHoleClient.Whoami()
}
func (w rabbitHoleClientWrapper) GetUsername() string {
	return w.rabbitHoleClient.Username
}
func (w rabbitHoleClientWrapper) SetUsername(username string) {
	w.rabbitHoleClient.Username = username
}
func (w rabbitHoleClientWrapper) SetPassword(passwd string) {
	w.rabbitHoleClient.Password = passwd
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/bobg/mid"
	"github.com/bobg/subcmd/v2"
	"github.com/google/go-github/v44/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gopkg.in/yaml.v3"

	"crocs"
	"crocs/sqlite"
)

func main() {
	var c maincmd
	err := subcmd.Run(context.Background(), c, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}

type maincmd struct{}

func (maincmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"serve", doServe, "run the crocs server", subcmd.Params(
			"-config", subcmd.String, "config.yml", "path to config file",
		),
		"admin", doAdmin, "send an admin command to a crocs server", subcmd.Params(
			"-url", subcmd.String, "", "base URL of crocs server",
			"-key", subcmd.String, "", "admin key",
			"command", subcmd.String, "", "command name",
		),
	)
}

type config struct {
	Certfile     string
	Database     string // xxx should have a "sqlite3:" prefix or something to select different backends
	GithubSecret []byte `yaml:"github_secret"`
	GithubURL    string `yaml:"github_url"`
	Keyfile      string
	Listen       string
	SlackSecret  string `yaml:"slack_secret"`
	SlackToken   string `yaml:"slack_token"`
	AdminKey     string `yaml:"admin_key"`
}

var defaultConfig = config{
	Database:  "crocs.db",
	GithubURL: "http://github.com",
	Listen:    ":3853",
}

func doServe(ctx context.Context, configPath string, _ []string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return errors.Wrap(err, "opening config file")
	}
	defer f.Close()

	c := defaultConfig
	err = yaml.NewDecoder(f).Decode(&c)
	if err != nil {
		return errors.Wrap(err, "parsing config file")
	}

	ghClient, err := github.NewEnterpriseClient(c.GithubURL, c.GithubURL, nil)
	if err != nil {
		log.Fatalf("Creating GitHub client: %s", err)
	}

	slackClient := slack.New(c.SlackToken)

	commentStore, userStore, closer, err := sqlite.Open(ctx, c.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer closer()

	s := &crocs.Service{
		AdminKey:    c.AdminKey,
		Comments:    commentStore,
		GHClient:    ghClient,
		GHSecret:    c.GithubSecret,
		SlackClient: slackClient,
		SlackSecret: c.SlackSecret,
		Users:       userStore,
	}

	mux := http.NewServeMux()
	mux.Handle("/github", mid.Err(s.OnGHWebhook))
	mux.Handle("/slack", mid.Err(s.OnSlackEvent))

	httpServer := &http.Server{
		Addr:    c.Listen,
		Handler: mux,
	}
	ch := make(chan struct{})

	mux.Handle("/admin", mid.JSON(s.OnAdmin(httpServer, ch)))

	if c.Certfile != "" && c.Keyfile != "" {
		err = httpServer.ListenAndServeTLS(c.Certfile, c.Keyfile)
	} else {
		err = httpServer.ListenAndServe()
	}

	<-ch

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func doAdmin(ctx context.Context, url, key, command string, _ []string) error {
	cmd := crocs.AdminCmd{
		Key:  key,
		Name: command,
	}
	enc, err := json.Marshal(cmd)
	if err != nil {
		return errors.Wrap(err, "marshaling command")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(enc))
	if err != nil {
		return errors.Wrap(err, "preparing request")
	}
	req.Header.Set("Content-Type", "application/json")
	var cl http.Client
	_, err = cl.Do(req)
	return errors.Wrap(err, "sending command to crocs service")
}

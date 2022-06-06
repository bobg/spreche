package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/bobg/mid"
	"github.com/bobg/subcmd/v2"
	"github.com/google/go-github/v44/github"
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
			"-ngrok", subcmd.Bool, false, "launch ngrok tunnel",
		),
		"admin", doAdmin, "send an admin command to a crocs server", subcmd.Params(
			"-url", subcmd.String, "", "base URL of crocs server",
			"-key", subcmd.String, "", "admin key",
			"command", subcmd.String, "", "command name",
		),
	)
}

type config struct {
	AdminKey           string `yaml:"admin_key"`
	Certfile           string
	Database           string // xxx should have a "sqlite3:" prefix or something to select different backends
	GithubSecret       string `yaml:"github_secret"`
	GithubURL          string `yaml:"github_url"`
	Keyfile            string
	Listen             string
	SlackClientSecret  string `yaml:"slack_client_secret"`
	SlackSigningSecret string `yaml:"slack_signing_secret"`
	SlackToken         string `yaml:"slack_token"`
}

var defaultConfig = config{
	Database:  "crocs.db",
	GithubURL: "http://github.com",
	Listen:    ":3853",
}

var portRegex = regexp.MustCompile(`:(\d+)$`)

func doServe(ctx context.Context, configPath string, ngrok bool, _ []string) error {
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

	channelStore, commentStore, userStore, closer, err := sqlite.Open(ctx, c.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer closer()

	s := &crocs.Service{
		AdminKey:           c.AdminKey,
		Channels:           channelStore,
		Comments:           commentStore,
		GHClient:           ghClient,
		GHSecret:           c.GithubSecret,
		SlackClient:        slackClient,
		SlackClientSecret:  c.SlackClientSecret,
		SlackSigningSecret: c.SlackSigningSecret,
		Users:              userStore,
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

	log.Printf("Listening on %s", httpServer.Addr)

	if ngrok {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		m := portRegex.FindStringSubmatch(httpServer.Addr)
		if len(m) == 0 {
			return fmt.Errorf("could not parse addr %s", httpServer.Addr)
		}
		ngrokCmd := exec.CommandContext(ctx, "ngrok", "http", m[1])
		err = ngrokCmd.Start()
		if err != nil {
			return errors.Wrap(err, "starting ngrok")
		}
		go func() {
			err := ngrokCmd.Wait()
			if err != nil {
				log.Printf("Error running ngrok: %s", err)
			}
		}()

		err = exec.CommandContext(ctx, "open", "http://localhost:4040").Run()
		if err != nil {
			return errors.Wrapf(err, "opening localhost:4040")
		}
	}

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
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/admin", bytes.NewReader(enc))
	if err != nil {
		return errors.Wrap(err, "preparing request")
	}
	req.Header.Set("Content-Type", "application/json")
	var cl http.Client
	resp, err := cl.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending command to crocs service")
	}
	defer resp.Body.Close()
	log.Printf("Response: %s", resp.Status)
	return nil
}

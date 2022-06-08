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
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v44/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gopkg.in/yaml.v3"

	"spreche"
	"spreche/sqlite"
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
		"serve", doServe, "run the spreche server", subcmd.Params(
			"-config", subcmd.String, "config.yml", "path to config file",
			"-ngrok", subcmd.Bool, false, "launch ngrok tunnel",
		),
		"admin", doAdmin, "send an admin command to a spreche server", subcmd.Params(
			"-url", subcmd.String, "", "base URL of spreche server",
			"-key", subcmd.String, "", "admin key",
		),
	)
}

type config struct {
	AdminKey             string `yaml:"admin_key"`
	Certfile             string
	Database             string // xxx should have a "sqlite3:" prefix or something to select different backends
	GithubPrivateKeyFile string `yaml:"github_private_key_file"`
	GithubSecret         string `yaml:"github_secret"`
	GithubAPIURL         string `yaml:"github_api_url"`    // "https://api.github.com/" or "https://HOST/api/v3/"
	GithubUploadURL      string `yaml:"github_upload_url"` // "https://uploads.github.com/" or "https://HOST/api/uploads/"
	Keyfile              string
	Listen               string
	SlackSigningSecret   string `yaml:"slack_signing_secret"`
	SlackToken           string `yaml:"slack_token"`
}

var defaultConfig = config{
	Database:        "spreche.db",
	GithubAPIURL:    "https://api.github.com/",
	GithubUploadURL: "https://uploads.github.com/",
	Listen:          ":3853",
}

var portRegex = regexp.MustCompile(`:(\d+)$`)

const (
	ghAppID          = 207677 // https://github.com/settings/apps/spreche
	ghInstallationID = 26242918
)

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

	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, ghAppID, ghInstallationID, c.GithubPrivateKeyFile)
	if err != nil {
		return errors.Wrapf(err, "reading GitHub private key from %s", c.GithubPrivateKeyFile)
	}
	itr.BaseURL = c.GithubAPIURL

	ghClient, err := github.NewEnterpriseClient(c.GithubAPIURL, c.GithubUploadURL, &http.Client{Transport: itr})
	if err != nil {
		log.Fatalf("Creating GitHub client: %s", err)
	}

	slackClient := slack.New(c.SlackToken)

	channelStore, commentStore, userStore, closer, err := sqlite.Open(ctx, c.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer closer()

	s := &spreche.Service{
		AdminKey:           c.AdminKey,
		Channels:           channelStore,
		Comments:           commentStore,
		GHClient:           ghClient,
		GHSecret:           c.GithubSecret,
		SlackClient:        slackClient,
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

func doAdmin(ctx context.Context, url, key string, args []string) error {
	cmd := spreche.AdminCmd{
		Key:  key,
		Args: args,
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
		return errors.Wrap(err, "sending command to spreche service")
	}
	defer resp.Body.Close()
	log.Printf("Response: %s", resp.Status)
	return nil
}

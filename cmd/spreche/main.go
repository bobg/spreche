package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/bobg/mid"
	"github.com/bobg/subcmd/v2"
	"github.com/pkg/errors"
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
	Database             string
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
	Database:        "sqlite3:spreche.db",
	GithubAPIURL:    "https://api.github.com/",
	GithubUploadURL: "https://uploads.github.com/",
	Listen:          ":3853",
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

	s := spreche.Service{
		AdminKey:           c.AdminKey,
		GHSecret:           c.GithubSecret,
		SlackSigningSecret: c.SlackSigningSecret,
	}

	dbparts := strings.SplitN(c.Database, ":", 2)
	if len(dbparts) < 2 {
		return fmt.Errorf("bad database config string %s", c.Database)
	}

	switch dbparts[0] {
	case "sqlite3":
		stores, err := sqlite.Open(ctx, dbparts[1])
		if err != nil {
			return errors.Wrap(err, "opening database")
		}
		defer stores.Close()
		s.Channels = stores.Channels
		s.Comments = stores.Comments
		s.Users = stores.Users

	case "postgresql":
		stores, err := sqlite.Open(ctx, dbparts[1])
		if err != nil {
			return errors.Wrap(err, "opening database")
		}
		defer stores.Close()
		s.Channels = stores.Channels
		s.Comments = stores.Comments
		s.Users = stores.Users

	default:
		return fmt.Errorf("unknown database type %s", dbparts[0])
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
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		io.Copy(os.Stdout, resp.Body)
	}
	return nil
}

package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bobg/subcmd/v2"
	"github.com/google/go-github/v45/github"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"

	"spreche"
)

func doLambda(ctx context.Context, args []string) error {
	return subcmd.Run(ctx, lambdacmd{}, args)
}

type lambdacmd struct {
	s *spreche.Service
}

func (l lambdacmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"github", l.doLambdaGitHub, "run the GitHub lambda", nil,
		"slack", l.doLambdaSlack, "run the Slack lambda", nil,
	)
}

func (l lambdacmd) doLambdaGitHub(ctx context.Context, _ []string) error {
	lambda.StartWithOptions(l.githubLambdaHandler, lambda.WithContext(ctx))
	return nil
}

func (l lambdacmd) doLambdaSlack(ctx context.Context, _ []string) error {
	lambda.StartWithOptions(l.slackLambdaHandler, lambda.WithContext(ctx))
	return nil
}

func (l lambdacmd) githubLambdaHandler(ctx context.Context, req events.APIGatewayV2HTTPRequest) error {
	signature := req.Headers[strings.ToLower(github.SHA256SignatureHeader)]
	if signature == "" {
		signature = req.Headers[strings.ToLower(github.SHA1SignatureHeader)]
	}

	var contentType string // xxx

	payload, err := github.ValidatePayloadFromBody(contentType, strings.NewReader(req.Body), signature, []byte(l.s.GHSecret))
	if err != nil {
		return errors.Wrap(err, "validating payload")
	}

	typ := req.Headers[strings.ToLower(github.EventTypeHeader)]

	return l.s.OnValidGHWebhook(ctx, typ, payload)
}

func (l lambdacmd) slackLambdaHandler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	h := make(http.Header)
	for k, v := range req.Headers {
		h[k] = []string{v}
	}
	sv, err := slack.NewSecretsVerifier(h, l.s.SlackSigningSecret)
	if err != nil {
		// xxx
	}
	_, err = sv.Write([]byte(req.Body))
	if err != nil {
		// xxx
	}
	if err = sv.Ensure(); err != nil {
		// xxx
	}
	// xxx
	var resp events.APIGatewayV2HTTPResponse
	return resp, nil
}

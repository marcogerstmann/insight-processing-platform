package ssm

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type SecretProvider struct {
	client *ssm.Client
}

func NewSecretProvider(ctx context.Context) (*SecretProvider, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &SecretProvider{client: ssm.NewFromConfig(cfg)}, nil
}

func (p *SecretProvider) Get(ctx context.Context, name string) (string, error) {
	out, err := p.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	if out.Parameter == nil || out.Parameter.Value == nil {
		return "", errors.New("parameter not found or empty")
	}
	return strings.TrimSpace(*out.Parameter.Value), nil
}

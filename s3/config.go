package s3

import (
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/flyteorg/stow"
	"github.com/pkg/errors"
)

// Kind represents the name of the location/storage type.
const Kind = "s3"

var (
	authTypeAccessKey = "accesskey"
	authTypeIAM       = "iam"
)

const (
	// ConfigAuthType is an optional argument that defines whether to use an IAM role or access key based auth
	ConfigAuthType = "auth_type"

	// ConfigAccessKeyID is one key of a pair of AWS credentials.
	ConfigAccessKeyID = "access_key_id"

	// ConfigSecretKey is one key of a pair of AWS credentials.
	ConfigSecretKey = "secret_key"

	// ConfigToken is an optional argument which is required when providing
	// credentials with temporary access.
	ConfigToken = "token"

	// ConfigRegion represents the region/availability zone of the session.
	ConfigRegion = "region"

	// ConfigEndpoint is optional config value for changing s3 endpoint
	// used for e.g. minio.io
	ConfigEndpoint = "endpoint"

	// ConfigDisableSSL is optional config value for disabling SSL support on custom endpoints
	// Its default value is "false", to disable SSL set it to "true".
	ConfigDisableSSL = "disable_ssl"

	// ConfigV2Signing is an optional config value for signing requests with the v2 signature.
	// Its default value is "false", to enable set to "true".
	// This feature is useful for s3-compatible blob stores -- ie minio.
	ConfigV2Signing = "v2_signing"
)

func init() {
	validatefn := func(config stow.Config) error {
		authType, ok := config.Config(ConfigAuthType)
		if !ok || authType == "" {
			authType = authTypeAccessKey
		}

		if !(authType == authTypeAccessKey || authType == authTypeIAM) {
			return errors.New("invalid auth_type")
		}

		if authType == authTypeAccessKey {
			_, ok := config.Config(ConfigAccessKeyID)
			if !ok {
				return errors.New("missing Access Key ID")
			}

			_, ok = config.Config(ConfigSecretKey)
			if !ok {
				return errors.New("missing Secret Key")
			}
		}
		return nil
	}
	makefn := func(config stow.Config) (stow.Location, error) {

		authType, ok := config.Config(ConfigAuthType)
		if !ok || authType == "" {
			authType = authTypeAccessKey
		}

		if !(authType == authTypeAccessKey || authType == authTypeIAM) {
			return nil, errors.New("invalid auth_type")
		}

		if authType == authTypeAccessKey {
			_, ok := config.Config(ConfigAccessKeyID)
			if !ok {
				return nil, errors.New("missing Access Key ID")
			}

			_, ok = config.Config(ConfigSecretKey)
			if !ok {
				return nil, errors.New("missing Secret Key")
			}
		}

		// Create a new client (s3 session)
		client, endpoint, err := newS3Client(config, "")
		if err != nil {
			return nil, err
		}

		// Create a location with given config and client (s3 session).
		loc := &location{
			config:         config,
			client:         client,
			customEndpoint: endpoint,
		}

		return loc, nil
	}

	kindfn := func(u *url.URL) bool {
		return u.Scheme == Kind
	}

	stow.Register(Kind, makefn, kindfn, validatefn)
}

// Attempts to create a session based on the information given.
func newS3Client(config stow.Config, region string) (client *s3.S3, endpoint string, err error) {
	authType, _ := config.Config(ConfigAuthType)
	accessKeyID, _ := config.Config(ConfigAccessKeyID)
	secretKey, _ := config.Config(ConfigSecretKey)
	token, _ := config.Config(ConfigToken)

	if authType == "" {
		authType = authTypeAccessKey
	}

	awsConfig := aws.NewConfig().
		WithHTTPClient(http.DefaultClient).
		WithMaxRetries(aws.UseServiceDefaultRetries).
		WithLogger(aws.NewDefaultLogger()).
		WithLogLevel(aws.LogOff).
		WithSleepDelay(time.Sleep)

	if region == "" {
		region, _ = config.Config(ConfigRegion)
	}
	if region != "" {
		awsConfig.WithRegion(region)
	} else {
		awsConfig.WithRegion("us-east-1")
	}

	if authType == authTypeAccessKey {
		awsConfig.WithCredentials(credentials.NewStaticCredentials(accessKeyID, secretKey, token))
	}

	endpoint, ok := config.Config(ConfigEndpoint)
	if ok {
		awsConfig.WithEndpoint(endpoint).
			WithS3ForcePathStyle(true)
	}

	disableSSL, ok := config.Config(ConfigDisableSSL)
	if ok && disableSSL == "true" {
		awsConfig.WithDisableSSL(true)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, "", err
	}
	if sess == nil {
		return nil, "", errors.New("creating the S3 session")
	}

	s3Client := s3.New(sess)

	usev2, ok := config.Config(ConfigV2Signing)
	if ok && usev2 == "true" {
		setv2Handlers(s3Client)
	}

	return s3Client, endpoint, nil
}

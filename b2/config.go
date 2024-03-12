package b2

import (
	"errors"
	"net/url"

	"github.com/flyteorg/stow"
	"gopkg.in/kothar/go-backblaze.v0"
)

// Config key constants.
const (
	ConfigAccountID      = "account_id"
	ConfigApplicationKey = "application_key"
	ConfigKeyID          = "application_key_id"
)

// Kind is the kind of Location this package provides.
const Kind = "b2"

func init() {
	validatefn := func(config stow.Config) error {
		_, ok := config.Config(ConfigApplicationKey)
		if !ok {
			return errors.New("missing application key")
		}
		accountID, _ := config.Config(ConfigAccountID)
		keyID, _ := config.Config(ConfigKeyID)
		if accountID == "" && keyID == "" {
			return errors.New("account ID or applicaton key ID needs to be set")
		}
		return nil
	}
	makefn := func(config stow.Config) (stow.Location, error) {
		if err := validatefn(config); err != nil {
			return nil, err
		}
		l := &location{
			config: config,
		}
		var err error
		l.client, err = newB2Client(l.config)
		if err != nil {
			return nil, err
		}
		return l, nil
	}
	kindfn := func(u *url.URL) bool {
		return u.Scheme == Kind
	}
	stow.Register(Kind, makefn, kindfn, validatefn)
}

func newB2Client(cfg stow.Config) (*backblaze.B2, error) {
	accountID, _ := cfg.Config(ConfigAccountID)
	applicationKey, _ := cfg.Config(ConfigApplicationKey)
	keyID, _ := cfg.Config(ConfigKeyID)

	client, err := backblaze.NewB2(backblaze.Credentials{
		AccountID:      accountID,
		ApplicationKey: applicationKey,
		KeyID:          keyID,
	})

	if err != nil {
		return nil, errors.New("Unable to create client")
	}

	err = client.AuthorizeAccount()
	if err != nil {
		return nil, errors.New("Unable to authenticate")
	}

	return client, nil
}

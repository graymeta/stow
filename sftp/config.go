package sftp

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/flyteorg/stow"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Kind represents the name of the location/storage type.
const Kind = "sftp"

const (
	// ConfigHost is the hostname or IP address to connect to.
	ConfigHost = "host"

	// ConfigPort is the numeric port number the ssh daemon is listening on.
	ConfigPort = "port"

	// ConfigUsername is the username of the user to connect as.
	ConfigUsername = "username"

	// ConfigPassword is the password use to authentiate with. Can be used instead
	// of a private key (if your server allows this).
	ConfigPassword = "password"

	// ConfigPrivateKey is a private ssh key to use to authenticate. These are the
	// bytes representing the key, not the path on disk to the key file.
	ConfigPrivateKey = "private_key"

	// ConfigPrivateKeyPassphrase is an optional passphrase to use with the private key.
	ConfigPrivateKeyPassphrase = "private_key_passphrase"

	// ConfigHostPublicKey is the public host key (in the same format as would be
	// found in the known_hosts file). If this is not specified, or is set to an
	// empty string, the host key validation will be disabled.
	ConfigHostPublicKey = "host_public_key"

	// ConfigBasePath is the path to the root folder on the remote server. It can be
	// relative to the user's home directory, or an absolute path. If not set, or
	// set to an empty string, the user's home directory will be used.
	ConfigBasePath = "base_path"
)

type conf struct {
	host      string
	port      int
	basePath  string
	sshConfig ssh.ClientConfig
}

func (c conf) Host() string {
	return fmt.Sprintf("%s:%d", c.host, c.port)
}

func parseConfig(config stow.Config) (*conf, error) {
	var c conf
	var ok bool

	c.host, ok = config.Config(ConfigHost)
	if !ok || c.host == "" {
		return nil, errors.New("invalid hostname")
	}

	port, ok := config.Config(ConfigPort)
	if !ok {
		return nil, errors.New("port not specified")

	}
	var err error
	c.port, err = strconv.Atoi(port)
	if err != nil || c.port < 1 {
		return nil, errors.New("invalid port configuration")
	}

	c.sshConfig.User, ok = config.Config(ConfigUsername)
	if !ok || c.sshConfig.User == "" {
		return nil, errors.New("invalid username")
	}

	// If a private key is specified, load it up and add it to the list of
	// authentication methods to attempt against the server.
	privKey, ok := config.Config(ConfigPrivateKey)
	if ok && privKey != "" {
		// If a passphrase for the key is specified, use it to unlock the key.
		passphrase, _ := config.Config(ConfigPrivateKeyPassphrase)
		var signer ssh.Signer
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privKey), []byte(passphrase))
			if err != nil {
				return nil, errors.Wrap(err, "parsing key with passphrase")
			}
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(privKey))
			if err != nil {
				return nil, errors.Wrap(err, "parsing key")
			}
		}
		c.sshConfig.Auth = append(c.sshConfig.Auth, ssh.PublicKeys(signer))
	}

	// If a password was specified, add password auth to the list of auth methods
	// to try.
	password, _ := config.Config(ConfigPassword)
	if password != "" {
		c.sshConfig.Auth = append(c.sshConfig.Auth, ssh.Password(password))
	}

	// Require at least 1 authentication method.
	if len(c.sshConfig.Auth) == 0 {
		return nil, errors.New("no authentication methods specified")
	}

	// Start by ignoring host keys. If a host key is specified in the configuration,
	// use that to validate the remote server.
	c.sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	if hostKey, ok := config.Config(ConfigHostPublicKey); ok && hostKey != "" {
		_, _, parsedHostKey, _, _, err := ssh.ParseKnownHosts([]byte(hostKey))
		if err != nil {
			return nil, errors.Wrap(err, "parsing host key")
		}

		c.sshConfig.HostKeyCallback = ssh.FixedHostKey(parsedHostKey)
	}

	c.basePath, ok = config.Config(ConfigBasePath)
	if !ok || c.basePath == "" {
		c.basePath = "."
	}

	return &c, nil
}

func init() {
	validatefn := func(config stow.Config) error {
		_, err := parseConfig(config)
		return err
	}
	makefn := func(config stow.Config) (stow.Location, error) {
		c, err := parseConfig(config)
		if err != nil {
			return nil, err
		}

		loc := &location{
			config: c,
		}

		// Connect to the remote server and perform the SSH handshake.
		loc.sshClient, err = ssh.Dial("tcp", c.Host(), &c.sshConfig)
		if err != nil {
			return nil, errors.Wrap(err, "ssh connection")
		}

		// Open an SFTP session over an existing ssh connection.
		loc.sftpClient, err = sftp.NewClient(loc.sshClient)
		if err != nil {
			// close the ssh connection if the sftp connection fails. This avoids leaking
			// the ssh connection.
			loc.Close()
			return nil, errors.Wrap(err, "sftp connection")
		}

		return loc, nil
	}

	kindfn := func(u *url.URL) bool {
		return u.Scheme == Kind
	}

	stow.Register(Kind, makefn, kindfn, validatefn)
}

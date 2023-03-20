package caddy_gcp_secret_manager

import (
	"encoding/json"
	"errors"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"io/fs"
	"os"
)

var (
	ErrInvalidJSON     = errors.New("invalid JSON")
	ErrCredentialsFile = errors.New("unable to read credentials file")
)

func (c CaddyGcpSecretManagerStorage) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "caddy.storage.gcp-secret-manager",
		New: func() caddy.Module {
			return new(CaddyGcpSecretManagerStorage)
		},
	}
}

type CaddyGcpSecretManagerStorage struct {
	storage         *SecretManagerStorage
	logger          *zap.Logger
	Fs              fs.FS
	ProjectID       string
	CredentialsFile string
}

func (c *CaddyGcpSecretManagerStorage) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {

		d.NextArg()
		if v := d.Val(); v != "" && v != "gcp-secret-manager" {
			c.ProjectID = v
		}

		for nest := d.Nesting(); d.NextBlock(nest); {
			switch v := d.Val(); v {
			case "project_id":
				if !d.NextArg() {
					return d.ArgErr()
				}

				if c.ProjectID == "" {
					c.ProjectID = d.Val()
				}
				break
			case "credentials_file":
				if !d.NextArg() {
					return d.ArgErr()
				}

				c.CredentialsFile = d.Val()
				break
			default:
				return d.Errf("Unknown option '%v'", v)
			}
		}

		if c.ProjectID == "" {
			return d.Errf("project_id must be set")
		}
	}

	return nil
}

func (c CaddyGcpSecretManagerStorage) Provision(context caddy.Context) error {
	c.logger = context.Logger(c)

	return nil
}

func (c CaddyGcpSecretManagerStorage) Validate() error {
	var files fs.FS
	var opts []option.ClientOption

	if c.Fs != nil {
		files = c.Fs
	} else {
		files = os.DirFS("/")
	}

	if c.CredentialsFile != "" {
		file, err := files.Open(c.CredentialsFile)

		if err != nil {
			return errors.Join(ErrCredentialsFile, err)
		}

		var buffer []byte

		if _, err := file.Read(buffer); err != nil {
			return errors.Join(ErrCredentialsFile, err)
		}

		if !json.Valid(buffer) {
			return errors.Join(ErrInvalidJSON, ErrCredentialsFile)
		}
	}

	storage, err := NewSecretManagerStorage(c.ProjectID, opts...)

	if err != nil {
		return err
	}

	c.storage = storage

	return nil
}

func (c CaddyGcpSecretManagerStorage) Cleanup() error {
	return c.storage.Close()
}

var _ caddy.Module = (*CaddyGcpSecretManagerStorage)(nil)
var _ caddy.Provisioner = (*CaddyGcpSecretManagerStorage)(nil)
var _ caddy.Validator = (*CaddyGcpSecretManagerStorage)(nil)
var _ caddy.CleanerUpper = (*CaddyGcpSecretManagerStorage)(nil)
var _ caddyfile.Unmarshaler = (*CaddyGcpSecretManagerStorage)(nil)

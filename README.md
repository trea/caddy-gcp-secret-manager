# caddy-gcp-secret-manager

This package provides a module for the [Caddy](https://github.com/caddyserver/caddy) web server to use [Google Cloud Platform's Secret Manager](https://cloud.google.com/secret-manager/) product to store
TLS certificates.

## Features

- Secure retrieval and storage of TLS certificates
- Clustering compatible - locks with TTLs are written to Secret Manager

## Usage

### Building

To use this module with Caddy, you'll need to create a custom build with [`xcaddy`](https://github.com/caddyserver/xcaddy).

```bash
xcaddy build --with github.com/trea/caddy-gcp-secret-manager
```

#### Via Docker

You can build this module (and others) into Caddy with `xcaddy` using Docker. The following example uses a multi-phase build to build the
Caddy binary with xcaddy, and then copies your newly built binary over the original binary in the core Caddy image.

```Dockerfile
FROM caddy:2.6.4-builder AS builder

RUN xcaddy --with github.com/trea/caddy-gcp-secret-manager

FROM caddy:2.6.4

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

<!-- TODO: Document Caddy download page https://caddyserver.com/download -->

## Caddyfile Configuration Examples

### Application Default Credentials
The Google Cloud Platform Client library used by this package uses [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials) by default
which can automatically configure authentication and authorization.

The following configuration would use Caddy's static file server to serve files in /var/www/html at https://example.com and the TLS certificate will be stored in GCP Secret Manager.

`your-project-id` should be replaced with the Project ID for your project from your Google Cloud Platform console.

```
{
    storage gcp-secret-manager your-project-id
}

example.com:443 {
    file_server /var/www/html
}
```

### Specified Credentials

If you need to point to a specific credentials file, you can do so in configuration as well by setting the `credentials_file` option nested
under the `storage` block.

```
{
    storage gcp-secret-manager your-project-id {
        credentials_file /mnt/gcp-credentials.json
    }
}

example.com:443 {
    file_server /var/www/html
}
```

## Use with certmagic

Because Caddy's TLS is built on top of [certmagic](https://github.com/caddyserver/certmagic), this package can be used with certmagic directly like so:

```go
package main

import (
	"log"
	"net/http"

	"github.com/caddyserver/certmagic"

	caddy_gcp_secret_manager "github.com/trea/caddy-gcp-secret-manager"
)

func main() {

	storage, err := caddy_gcp_secret_manager.NewSecretManagerStorage("my-gcp-project")

	if err != nil {
		log.Fatalf("Unable to initialize storage: %+v", err)
	}

	// read and agree to your CA's legal documents
	// provide an email address
	// use the staging endpoint while we're developing
	//
	// uncomment and change the values on the following lines as applicable

	//certmagic.DefaultACME.Agreed = true
	//certmagic.DefaultACME.Email = "you@yours.com"
	//certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	
	certmagic.Default.Storage = storage

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello world!"))
	})

	if err := certmagic.HTTPS([]string{"example.com"}, mux); err != nil {
		log.Fatalf("Unable to start listener: %+v", err)
	}
}
```

`NewSecretManagerStorage` also accepts [`option.ClientOption`](https://pkg.go.dev/google.golang.org/api/option#ClientOption) if you need to alter connection configuration as shown in the client
[examples](https://github.com/googleapis/google-cloud-go#authorization).
package s3

import "github.com/aws/aws-sdk-go-v2/service/s3"

// Option configures an S3-compatible storage driver.
type Option func(*options)

type options struct {
	name         string
	bucket       string
	region       string
	endpoint     string
	accessKey    string
	secretKey    string
	sessionToken string
	domain       string
	urlPrefix    string
	pathStyle    bool
	client       *s3.Client
	presigner    *s3.PresignClient
}

func defaultOptions() options { return options{name: "s3", region: "auto"} }

// Name sets driver name metadata.
func Name(value string) Option { return func(options *options) { options.name = value } }

// Bucket sets the S3 bucket.
func Bucket(value string) Option { return func(options *options) { options.bucket = value } }

// Region sets the signing region.
func Region(value string) Option { return func(options *options) { options.region = value } }

// Endpoint sets an S3-compatible endpoint for MinIO/R2/OSS.
func Endpoint(value string) Option { return func(options *options) { options.endpoint = value } }

// Credentials sets static credentials.
func Credentials(accessKey string, secretKey string, sessionToken ...string) Option {
	return func(options *options) {
		options.accessKey = accessKey
		options.secretKey = secretKey
		if len(sessionToken) > 0 {
			options.sessionToken = sessionToken[0]
		}
	}
}

// Domain sets public URL domain.
func Domain(value string) Option { return func(options *options) { options.domain = value } }

// URLPrefix sets public URL prefix.
func URLPrefix(value string) Option { return func(options *options) { options.urlPrefix = value } }

// PathStyle forces path-style addressing for MinIO-like services.
func PathStyle(value bool) Option { return func(options *options) { options.pathStyle = value } }

// Client uses an existing AWS S3 client.
func Client(client *s3.Client) Option { return func(options *options) { options.client = client } }

// Presigner uses an existing S3 presign client.
func Presigner(client *s3.PresignClient) Option {
	return func(options *options) { options.presigner = client }
}

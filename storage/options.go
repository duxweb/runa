package storage

import "github.com/duxweb/runa/core"

// DiskOption configures a named disk.
type DiskOption interface {
	ApplyDisk(*DiskOptions)
}

// FileOption configures one file operation.
type FileOption interface {
	ApplyFile(*FileOptions)
}

// DriverOption configures local or external driver wrappers.
type DriverOption interface {
	ApplyDriver(*DriverOptions)
}

// DiskOptions stores named disk configuration.
type DiskOptions struct {
	Driver    string
	Prefix    string
	Public    bool
	URLPrefix string
	Domain    string
	Meta      core.Map
}

// FileOptions stores file operation metadata.
type FileOptions struct {
	ContentType string
	Meta        core.Map
}

// URLOptions stores URL generation options.
type URLOptions struct {
	Public    bool
	Prefix    string
	URLPrefix string
	Domain    string
	Meta      core.Map
}

// DriverOptions stores low-level driver configuration.
type DriverOptions struct {
	Name      string
	Root      string
	Domain    string
	URLPrefix string
	Secret    string
	Meta      core.Map
}

type diskOptionFunc func(*DiskOptions)
type fileOptionFunc func(*FileOptions)
type driverOptionFunc func(*DriverOptions)

func (fn diskOptionFunc) ApplyDisk(options *DiskOptions)       { fn(options) }
func (fn fileOptionFunc) ApplyFile(options *FileOptions)       { fn(options) }
func (fn driverOptionFunc) ApplyDriver(options *DriverOptions) { fn(options) }

// Driver selects the storage driver used by a disk.
func Use(name string) DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.Driver = name })
}

// Prefix sets a disk or driver path prefix.
func Prefix(value string) DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.Prefix = cleanPrefix(value) })
}

// Public marks a disk as publicly addressable.
func Public() DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.Public = true })
}

// Private marks a disk as private.
func Private() DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.Public = false })
}

// URLPrefix sets the public URL prefix for this disk.
func URLPrefix(value string) DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.URLPrefix = cleanURLPrefix(value) })
}

// Domain sets the public domain for this disk.
func Domain(value string) DiskOption {
	return diskOptionFunc(func(options *DiskOptions) { options.Domain = cleanDomain(value) })
}

// Meta stores arbitrary disk metadata.
func Meta(key string, value any) DiskOption {
	return diskOptionFunc(func(options *DiskOptions) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

// ContentType sets content type for one write/sign operation.
func ContentType(value string) FileOption {
	return fileOptionFunc(func(options *FileOptions) { options.ContentType = value })
}

// FileMeta stores arbitrary file operation metadata.
func FileMeta(key string, value any) FileOption {
	return fileOptionFunc(func(options *FileOptions) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

// Name sets driver name metadata.
func Name(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Name = value })
}

// Root sets local driver root path.
func Root(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Root = value })
}

// DriverDomain sets local driver URL domain.
func DriverDomain(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Domain = cleanDomain(value) })
}

// DriverURLPrefix sets local driver URL prefix.
func DriverURLPrefix(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.URLPrefix = cleanURLPrefix(value) })
}

// Secret sets signing secret for drivers that support signed URLs.
func Secret(value string) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) { options.Secret = value })
}

// DriverMeta stores arbitrary driver metadata.
func DriverMeta(key string, value any) DriverOption {
	return driverOptionFunc(func(options *DriverOptions) {
		if options.Meta == nil {
			options.Meta = make(core.Map)
		}
		options.Meta[key] = value
	})
}

func applyDiskOptions(options ...DiskOption) DiskOptions {
	opts := DiskOptions{Driver: DefaultDriver, Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyDisk(&opts)
		}
	}
	if opts.Driver == "" {
		opts.Driver = DefaultDriver
	}
	opts.Prefix = cleanPrefix(opts.Prefix)
	opts.URLPrefix = cleanURLPrefix(opts.URLPrefix)
	opts.Domain = cleanDomain(opts.Domain)
	return opts
}

func applyFileOptions(options ...FileOption) FileOptions {
	opts := FileOptions{Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyFile(&opts)
		}
	}
	return opts
}

func applyDriverOptions(options ...DriverOption) DriverOptions {
	opts := DriverOptions{Name: DefaultDriver, Root: ".", Meta: make(core.Map)}
	for _, option := range options {
		if option != nil {
			option.ApplyDriver(&opts)
		}
	}
	if opts.Name == "" {
		opts.Name = DefaultDriver
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	opts.URLPrefix = cleanURLPrefix(opts.URLPrefix)
	opts.Domain = cleanDomain(opts.Domain)
	return opts
}

func fileOptionsWithContentType(options FileOptions, contentType string) FileOptions {
	if options.ContentType == "" {
		options.ContentType = contentType
	}
	if options.Meta == nil {
		options.Meta = make(core.Map)
	}
	return options
}

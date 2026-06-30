package validate

// FieldSchema stores input field metadata collected from struct tags.
type FieldSchema struct {
	Source      string
	Name        string
	Field       string
	Label       string
	Description string
	Default     string
	Type        string
	Format      string
}

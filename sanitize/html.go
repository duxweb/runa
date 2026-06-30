package sanitize

// HTML sanitizes input using the first supplied policy or Strict by default.
func HTML(input string, policies ...Policy) string {
	policy := Strict()
	if len(policies) > 0 && policies[0] != nil {
		policy = policies[0]
	}
	return policy.Sanitize(input)
}

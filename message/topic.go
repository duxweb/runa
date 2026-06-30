package message

import "strings"

func MatchTopic(pattern string, topic string) bool {
	if pattern == topic {
		return true
	}
	if pattern == "" || topic == "" {
		return false
	}
	return matchSegments(splitTopic(pattern), splitTopic(topic))
}

func splitTopic(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '/' || r == ':' })
}

func matchSegments(pattern []string, topic []string) bool {
	for len(pattern) > 0 {
		head := pattern[0]
		pattern = pattern[1:]
		if head == "#" || head == "**" {
			return true
		}
		if len(topic) == 0 {
			return false
		}
		if head != "*" && head != "+" && head != topic[0] {
			return false
		}
		topic = topic[1:]
	}
	return len(topic) == 0
}

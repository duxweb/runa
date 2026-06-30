package ws

// ByID filters clients by id.
func ByID(id string) Filter { return func(client ClientInfo) bool { return client.ID == id } }

// ByChannel filters clients by channel.
func ByChannel(channel string) Filter {
	return func(client ClientInfo) bool {
		for _, item := range client.Channels {
			if item == channel {
				return true
			}
		}
		return false
	}
}

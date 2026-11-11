package portals

// ChainByKey returns the chain registered under key.
func ChainByKey(key string) (Chain, bool) {
	for _, c := range Chains {
		if c.Key == key {
			return c, true
		}
	}
	return Chain{}, false
}

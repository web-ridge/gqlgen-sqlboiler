package cache

func AppendIfMissing(slice []string, v string) []string {
	if SliceContains(slice, v) {
		return slice
	}
	return append(slice, v)
}

func SliceContains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

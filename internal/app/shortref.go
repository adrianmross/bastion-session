package app

import "sort"

// BuildShortRefs returns a stable, unique short reference for each ID.
// It prefers short suffixes (minLen first) and increases length until uniqueness is achieved.
// IDs that still collide at maximum tested length fall back to full ID.
func BuildShortRefs(ids []string, minLen int) map[string]string {
	if minLen < 1 {
		minLen = 1
	}
	refs := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return refs
	}
	remaining := make(map[string]struct{}, len(ids))
	maxLen := minLen
	for _, id := range ids {
		remaining[id] = struct{}{}
		if len(id) > maxLen {
			maxLen = len(id)
		}
	}

	for l := minLen; l <= maxLen && len(remaining) > 0; l++ {
		buckets := map[string][]string{}
		for id := range remaining {
			ref := suffix(id, l)
			buckets[ref] = append(buckets[ref], id)
		}
		for ref, bucket := range buckets {
			if len(bucket) == 1 {
				id := bucket[0]
				refs[id] = ref
				delete(remaining, id)
			}
		}
	}

	if len(remaining) > 0 {
		idsLeft := make([]string, 0, len(remaining))
		for id := range remaining {
			idsLeft = append(idsLeft, id)
		}
		sort.Strings(idsLeft)
		for _, id := range idsLeft {
			refs[id] = id
		}
	}
	return refs
}

func suffix(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

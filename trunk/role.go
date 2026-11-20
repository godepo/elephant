package trunk

const defaultRolesLabelsSize = 3

func (r roleRules) init() roleRules {
	labels := make([]string, 0, defaultRolesLabelsSize)

	if r.shardID != "" {
		labels = append(labels, r.shardID)
	}

	if r.role != "" {
		labels = append(labels, r.role)
	}

	if r.id != "" {
		labels = append(labels, r.id)
	}

	r.labelsValues = labels

	return r
}

func (r roleRules) StatsLabels() []string {
	labels := make([]string, 0, defaultRolesLabelsSize)

	if r.shardID != "" {
		labels = append(labels, r.shardID)
	}

	if r.role != "" {
		labels = append(labels, r.role)
	}

	if r.id != "" {
		labels = append(labels, r.id)
	}

	return labels
}

func (r roleRules) Values(labels []string) []string {
	if len(r.labelsValues) == 0 {
		return labels
	}

	out := make([]string, len(labels), len(labels)+defaultRolesLabelsSize)

	copy(out, labels)

	return append(out, r.labelsValues...)
}

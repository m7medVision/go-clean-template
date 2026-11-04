package eventstore

// SnapshotDue reports whether a stream that grew from version before to version after crossed a
// multiple of every, and so is due a snapshot. An every below 1 never snapshots.
func SnapshotDue(before, after, every int) bool {
	if every < 1 {
		return false
	}

	return after/every > before/every
}

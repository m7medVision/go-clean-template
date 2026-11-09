package projection

import "time"

func SetCheckpointGrace(r *Runner, d time.Duration) { r.grace = d }

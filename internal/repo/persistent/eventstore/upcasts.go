package eventstore

// Upcasts lists how each event type's older stored versions become its current one. An event whose
// shape changes gets a new SchemaVersion and an Upcast here from the version before; the stored
// events stay as they were written. Every event is at its first version, so there are none yet.
func Upcasts() []Upcast {
	return nil
}

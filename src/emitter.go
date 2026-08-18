package main

type Event struct {
	Command string
	Data    any
}

type Emitter struct {
	nextListenerID uint64
	Listeners      map[string]map[uint64]func(*Event)
}

/* Event functions */

func (emitter *Emitter) On(command string, listener func(*Event)) uint64 {
	id := emitter.nextListenerID
	emitter.nextListenerID++

	lMap, exists := emitter.Listeners[command]
	if !exists {
		lMap = make(map[uint64]func(*Event))
		emitter.Listeners[command] = lMap
	}

	lMap[id] = listener

	return id
}

func (emitter *Emitter) Off(command string, listenerId uint64) {
	lMap, exists := emitter.Listeners[command]
	if exists {
		delete(lMap, listenerId)
	}
}

func (emitter *Emitter) Clear(command string) {
	delete(emitter.Listeners, command)
}

func (emitter *Emitter) ClearAll() {
	clear(emitter.Listeners)
}

func (emitter *Emitter) Emit(ev Event) {
	lMap, exists := emitter.Listeners[ev.Command]
	if exists {
		for _, value := range lMap {
			value(&ev)
		}
	}
}

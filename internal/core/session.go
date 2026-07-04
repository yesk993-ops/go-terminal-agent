package core

type SessionStore interface {
	Create() Session
	Get(id string) (Session, bool)
	GetOrCreate(id string) Session
	Delete(id string)
	List() []Session
}

type Session interface {
	ID() string
	Messages() []Message
	Append(msg Message)
	Clear()
	SetMetadata(key, value string)
	GetMetadata(key string) (string, bool)
}

package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Mongo wraps a MongoDB client and the application database handle.
type Mongo struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// NewMongo connects to MongoDB, verifies the connection with a ping and returns
// a ready-to-use handle to the given database.
func NewMongo(ctx context.Context, uri, dbName string) (*Mongo, error) {
	if uri == "" {
		return nil, fmt.Errorf("store: empty mongo uri")
	}
	if dbName == "" {
		return nil, fmt.Errorf("store: empty mongo database name")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("store: mongo connect: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("store: mongo ping: %w", err)
	}

	return &Mongo{
		Client: client,
		DB:     client.Database(dbName),
	}, nil
}

// Collection returns the named collection from the application database.
func (m *Mongo) Collection(name string) *mongo.Collection {
	return m.DB.Collection(name)
}

// Close disconnects the underlying client.
func (m *Mongo) Close(ctx context.Context) error {
	if m == nil || m.Client == nil {
		return nil
	}
	return m.Client.Disconnect(ctx)
}

// IsDuplicateKey reports whether err is a MongoDB duplicate-key (E11000) error.
func IsDuplicateKey(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

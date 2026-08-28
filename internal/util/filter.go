package util

import (
	"go.mongodb.org/mongo-driver/v2/bson"
)

type FilterBuilder struct {
	res bson.D
}

func NewFilter() *FilterBuilder {
	return &FilterBuilder{
		res: bson.D{},
	}
}

func (b *FilterBuilder) AppendIf(condition bool, field string, value any) *FilterBuilder {
	if condition {
		b.res = append(b.res, bson.E{Key: field, Value: value})
	}
	return b
}

func (b *FilterBuilder) Bson() bson.D {
	return b.res
}

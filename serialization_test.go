package apd

import (
	"github.com/globalsign/mgo/bson"
	"testing"
)

func TestDecimal_BSON(t *testing.T) {
	type XXX struct {
		Value *Decimal
	}

	var x = XXX{Value: new(Decimal).SetInt64(1234)}

	data, err := bson.Marshal(x)

	if err != nil {
		t.Error("marshal bson:", err)
		return
	}

	var y XXX
	err = bson.Unmarshal(data, &y)
	if err != nil {
		t.Error("unmarshal bson:", err)
		return
	}
	if x.Value.Cmp(y.Value) != 0 {
		t.Error("bson marshal/unmarshal not equal:", x, "!=", y)
		return
	}
}

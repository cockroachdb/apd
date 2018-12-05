package apd

import (
	"github.com/globalsign/mgo/bson"
)

// Convert data to Decimal128 type
func (d *Decimal) GetBSON() (interface{}, error) {
	return bson.ParseDecimal128(d.String())
}

// Parse from Decimal128 type
func (d *Decimal) SetBSON(raw bson.Raw) error {
	var w bson.Decimal128
	err := raw.Unmarshal(&w)
	if err != nil {
		return err
	}
	_, _, err = d.SetString(w.String())
	return err
}

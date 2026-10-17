package importer

import (
	"reflect"
	"time"

	"gorm.io/gorm"
)

// Cachet writes its dates in the timezone of its APP_TIMEZONE variable, UTC
// by default, without a zone: a DATETIME column holds wall-clock time. That
// variable lives in Cachet's .env, not in its database, so the importer is
// told which zone to read the dates in.

const zoneKey = "importer:source-zone"

// inSourceZone makes every query on the source database read its dates as
// wall-clock times in loc. The drivers return them as UTC.
func inSourceZone(src *gorm.DB, loc *time.Location) (*gorm.DB, error) {
	if loc == nil || loc == time.UTC {
		return src, nil
	}
	if src.Callback().Query().Get(zoneKey) == nil {
		err := src.Callback().Query().After("gorm:query").Register(zoneKey, func(tx *gorm.DB) {
			if l, ok := tx.Get(zoneKey); ok && tx.Error == nil {
				setZone(reflect.ValueOf(tx.Statement.Dest), l.(*time.Location))
			}
		})
		if err != nil {
			return nil, err
		}
	}
	// Session: the instance Set returns would carry each query's conditions
	// into the next one.
	return src.Set(zoneKey, loc).Session(&gorm.Session{}), nil
}

var timeType = reflect.TypeOf(time.Time{})

// setZone rewrites, in place, every time.Time reachable from v: the same
// wall-clock time, in loc.
func setZone(v reflect.Value, loc *time.Location) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			setZone(v.Elem(), loc)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			setZone(v.Index(i), loc)
		}
	case reflect.Struct:
		if v.Type() == timeType {
			if v.CanSet() {
				t := v.Interface().(time.Time)
				if !t.IsZero() {
					v.Set(reflect.ValueOf(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)))
				}
			}
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				setZone(v.Field(i), loc)
			}
		}
	}
}

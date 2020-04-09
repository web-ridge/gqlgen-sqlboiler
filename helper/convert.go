package helper

import (
	"strconv"
	"strings"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/iancoleman/strcase"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/types"
)

func IntsToInterfaces(ints []int) []interface{} {
	interfaces := make([]interface{}, len(ints))
	for index, number := range ints {
		interfaces[index] = number
	}
	return interfaces
}

func IDsToBoilerInterfaces(ids []string) []interface{} {
	interfaces := make([]interface{}, len(ids))
	for index, id := range ids {
		interfaces[index] = IDToBoiler(id)
	}
	return interfaces
}

func IDsToBoiler(ids []string) []uint {
	ints := make([]uint, len(ids))
	for index, stringID := range ids {
		ints[index] = IDToBoiler(stringID)
	}
	return ints
}

func IDToBoiler(ID string) uint {
	splitted := strings.Split(ID, "-")
	if len(splitted) > 1 {
		// nolint: errcheck
		i, _ := strconv.ParseUint(splitted[1], 10, 64)
		return uint(i)
	}
	return 0
}

func IDToNullBoiler(ID string) null.Uint {
	uintID := IDToBoiler(ID)
	if uintID == 0 {
		return null.NewUint(0, false)
	}
	return null.Uint{
		Uint:  uintID,
		Valid: false,
	}
}

func IDToGraphQL(id uint, tableName string) string {
	return strcase.ToLowerCamel(tableName) + "-" + strconv.Itoa(int(id))
}

func NullDotBoolToPointerBool(v null.Bool) *bool {
	return v.Ptr()
}

func NullDotStringToPointerString(v null.String) *string {
	return v.Ptr()
}

func NullDotTimeToPointerInt(v null.Time) *int {
	pv := v.Ptr()
	if pv == nil {
		return nil
	}
	u := int(pv.Unix())
	return &u
}

func TimeTimeToInt(v time.Time) int {
	return int(v.Unix())
}

func IntToTimeTime(v int) time.Time {
	return time.Unix(int64(v), 0)
}

func NullDotStringToString(v null.String) string {
	if v.Ptr() == nil {
		return ""
	}
	return *v.Ptr()
}

func NullDotUintToPointerInt(v null.Uint) *int {
	pv := v.Ptr()
	if pv == nil {
		return nil
	}
	u := int(*pv)
	return &u
}

func PointerStringToString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func PointerIntToNullDotTime(v *int) null.Time {
	return null.TimeFrom(time.Unix(int64(*v), 0))
}

func StringToNullDotString(v string) null.String {
	return null.StringFrom(v)
}

func PointerStringToNullDotString(v *string) null.String {
	return null.StringFromPtr(v)
}

func PointerBoolToNullDotBool(v *bool) null.Bool {
	return null.BoolFromPtr(v)
}

func TypesNullDecimalToFloat64(v types.NullDecimal) float64 {
	f, _ := v.Float64()
	return f
}

func Float64ToTypesNullDecimal(v float64) types.NullDecimal {
	d := new(decimal.Big)
	d.SetFloat64(v)
	return types.NewNullDecimal(d)
}

func TypesNullDecimalToPointerString(v types.NullDecimal) *string {
	s := v.String()
	if s == "" {
		return nil
	}
	return &s
}

func PointerStringToTypesNullDecimal(v *string) types.NullDecimal {
	if v == nil {
		return types.NewNullDecimal(nil)
	}
	d := new(decimal.Big)
	if _, ok := d.SetString(*v); !ok {
		nd := types.NewNullDecimal(nil)
		if err := d.Context.Err(); err != nil {
			return nd
		}
		// TODO: error handling maybe write log line here
		// https://github.com/volatiletech/sqlboiler/blob/master/types/decimal.go#L156
		return nd
	}

	return types.NewNullDecimal(d)
}

func PointerIntToNullDotInt(v *int) null.Int {
	return null.IntFromPtr((v))
}

func PointerIntToNullDotUint(v *int) null.Uint {
	if v == nil {
		return null.UintFromPtr(nil)
	}
	uv := *v
	return null.UintFrom(uint(uv))
}

func NullDotIntToPointerInt(v null.Int) *int {
	return v.Ptr()
}

func IntToInt8(v int) int8 {
	return int8(v)
}

func Int8ToInt(v int8) int {
	return int(v)
}

func NullDotFloat64ToPointerFloat64(v null.Float64) *float64 {
	return v.Ptr()
}

func PointerFloat64ToNullDotFloat64(v *float64) null.Float64 {
	return null.Float64FromPtr(v)
}

func IntToUint(v int) uint {
	return uint(v)
}

func UintToInt(v uint) int {
	return int(v)
}

func BoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func IntToBool(v int) bool {
	return v == 1
}

func NullDotBoolToPointerInt(v null.Bool) *int {
	pv := v.Ptr()
	if pv == nil {
		return nil
	}
	if *pv {
		i := 1
		return &i
	}
	i := 0
	return &i
}

func PointerIntToNullDotBool(v *int) null.Bool {
	if v == nil {
		return null.Bool{
			Valid: false,
		}
	}
	return null.Bool{
		Valid: v != nil,
		Bool:  *v == 1,
	}
}

func NullDotIntToUint(v null.Int) uint {
	return uint(v.Int)
}

func NullDotUintToUint(v null.Uint) uint {
	return v.Uint
}

func NullDotIntIsFilled(v null.Int) bool {
	return !v.IsZero()
}

func NullDotUintIsFilled(v null.Uint) bool {
	return !v.IsZero()
}

func UintIsFilled(v uint) bool {
	return v != 0
}

func IntIsFilled(v int) bool {
	return v != 0
}

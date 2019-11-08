package helper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/types"
)

func StringToIntID(ID string, entityName string) int {
	i, _ := strconv.ParseInt(strings.TrimPrefix(ID, entityName+"_"), 10, 64)
	return int(i)
}

func IntToStringIDUnique(id int, entityName string) string {
	return entityName + "_" + strconv.Itoa(id)
}

func StringToUintID(ID string, entityName string) uint {
	i, _ := strconv.ParseUint(strings.TrimPrefix(ID, entityName+"_"), 10, 64)
	return uint(i)
}

func UintToStringIDUnique(id uint, entityName string) string {
	return entityName + "_" + strconv.Itoa(int(id))
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

func PointerIntToNullDotTime(v *int) null.Time {
	if v == nil {
		return null.Time{
			Valid: false,
		}
	}
	return null.Time{
		Valid: true,
		Time:  time.Unix(int64(*v), 0),
	}
}
func StringToNullDotString(v string) null.String {
	// TODO nullable?? what to here
	return null.String{
		Valid:  true,
		String: v,
	}
}
func PointerStringToNullDotString(v *string) null.String {
	if v == nil {
		return null.String{
			Valid: false,
		}
	}
	return null.String{
		Valid:  v != nil,
		String: *v,
	}
}

func PointerBoolToNullDotBool(v *bool) null.Bool {
	if v == nil {
		return null.Bool{
			Valid: false,
		}
	}
	return null.Bool{
		Valid: v != nil,
		Bool:  *v,
	}
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
	// TODO
	fmt.Println("not implemented convert")

	if v == nil {
		return null.Int{}
	}
	return null.Int{}
}
func PointerIntToNullDotUint(v *int) null.Uint {
	// TODO
	fmt.Println("not implemented convert")

	if v == nil {
		return null.Uint{}
	}
	return null.Uint{}
}
func NullDotIntToPointerInt(v null.Int) *int {
	// TODO
	fmt.Println("not implemented convert")

	return nil
}
func IntToInt8(v int) int8 {
	return int8(v)
}
func Int8ToInt(v int8) int {
	return int(v)
}
func NullDotFloat64ToPointerFloat64(v null.Float64) *float64 {
	// TODO
	fmt.Println("not implemented convert")

	return nil

}
func PointerFloat64ToNullDotFloat64(v *float64) null.Float64 {
	// TODO
	fmt.Println("not implemented convert")

	if v == nil {
		return null.Float64{}
	}
	return null.Float64{}
}

func IntToUint(v int) uint {
	return uint(v)
}

func UintToInt(v uint) int {
	return int(v)
}

func BoolToInt(v bool) int {
	if v == true {
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
	if *pv == true {
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
func NullDotIntIsZero(v null.Int) bool {
	return v.IsZero()
}
func NullDotUintIsZero(v null.Uint) bool {
	return v.IsZero()
}
func UintIsZero(v uint) bool {
	return v == 0
}
func IntIsZero(v int) bool {
	return v == 0
}

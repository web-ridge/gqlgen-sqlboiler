package structs

import (
	"go/types"

	"github.com/vektah/gqlparser/v2/ast"
)

type ConvertConfig struct {
	IsCustom         bool
	ToBoiler         string
	ToGraphQL        string
	GraphTypeAsText  string
	BoilerTypeAsText string
}

type Interface struct {
	Description string
	Name        string
}

type Preload struct {
	Key           string
	ColumnSetting ColumnSetting
}

type Model struct { //nolint:maligned
	Name               string
	JSONName           string
	PluralName         string
	BoilerModel        *BoilerModel
	HasBoilerModel     bool
	PrimaryKeyType     string
	Fields             []*Field
	IsNormal           bool
	IsInput            bool
	IsCreateInput      bool
	IsUpdateInput      bool
	IsNormalInput      bool
	IsPayload          bool
	IsConnection       bool
	IsEdge             bool
	IsOrdering         bool
	IsWhere            bool
	IsFilter           bool
	IsPreloadable      bool
	PreloadArray       []Preload
	HasDeletedAt       bool
	HasPrimaryStringID bool
	// other stuff
	Description           string
	PureFields            []*ast.FieldDefinition
	Implements            []string
	TableNameResolverName string
}

type ColumnSetting struct {
	Name                  string
	RelationshipModelName string
	IDAvailable           bool
}

type Field struct { //nolint:maligned
	Name               string
	JSONName           string
	PluralName         string
	Type               string
	TypeWithoutPointer string
	IsNumberID         bool
	IsPrimaryNumberID  bool
	IsPrimaryStringID  bool
	IsPrimaryID        bool
	IsRequired         bool
	IsPlural           bool
	ConvertConfig      ConvertConfig
	Enum               *Enum
	// relation stuff
	IsRelation                 bool
	IsRelationAndNotForeignKey bool
	IsObject                   bool
	// boiler relation stuff is inside this field
	BoilerField BoilerField
	// graphql relation ship can be found here
	Relationship  *Model
	IsOr          bool
	IsAnd         bool
	IsWithDeleted bool

	// Some stuff
	Description  string
	OriginalType types.Type
}

type Enum struct {
	Description   string
	Name          string
	PluralName    string
	Values        []*EnumValue
	HasBoilerEnum bool
	HasFilter     bool
	BoilerEnum    *BoilerEnum
}

type EnumValue struct {
	Description     string
	Name            string
	NameLower       string
	BoilerEnumValue *BoilerEnumValue
}

type BoilerModel struct {
	Name               string
	TableName          string
	PluralName         string
	Fields             []*BoilerField
	Enums              []*BoilerEnum
	HasPrimaryStringID bool
	HasDeletedAt       bool
	IsView             bool
}

type BoilerField struct {
	Name             string
	PluralName       string
	Type             string
	IsForeignKey     bool
	IsRequired       bool
	IsArray          bool
	IsEnum           bool
	IsRelation       bool
	InTable          bool
	InTableNotID     bool
	Enum             BoilerEnum
	RelationshipName string
	Relationship     *BoilerModel
}

type BoilerEnum struct {
	Name          string
	ModelName     string
	ModelFieldKey string
	Values        []*BoilerEnumValue
}

type BoilerEnumValue struct {
	Name string
}

type BoilerType struct {
	Name string
	Type string
}

type Config struct {
	Directory   string
	PackageName string
}

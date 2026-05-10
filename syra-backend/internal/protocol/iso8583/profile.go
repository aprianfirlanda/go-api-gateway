package iso8583

type FieldType string

const (
	FieldFixed  FieldType = "fixed"
	FieldLLVAR  FieldType = "llvar"
	FieldLLLVAR FieldType = "lllvar"
)

type Profile struct {
	ID            string
	Name          string
	MTI           string
	ResponseMTI   string
	LengthHeader  bool
	Fields        map[int]FieldSpec
	SensitiveKeys []string
}

type FieldSpec struct {
	ID        int
	Type      FieldType
	Length    int
	Sensitive bool
}

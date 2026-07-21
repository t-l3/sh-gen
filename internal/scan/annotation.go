package scan

type AnnotationType int

const (
	CommandAnnotation AnnotationType = iota
	FlagAnnotation
)

type Annotation struct {
	Type AnnotationType

}
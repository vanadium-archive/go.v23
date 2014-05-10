package vom

// structSet hash conses vStructType objects
type structSet struct {
	nodeVal Type
	m       map[StructField]*structSet
}

func newStructSet() *structSet {
	return &structSet{m: make(map[StructField]*structSet)}
}

func (ss *structSet) HashCons(typ Type) Type {
	return ss.hashConsInternal(typ.(*vStructType).fields, typ)
}

func (ss *structSet) hashConsInternal(fields []StructField, typ Type) Type {
	if len(fields) == 0 {
		if ss.nodeVal == nil {
			ss.nodeVal = typ
		}
		return ss.nodeVal
	}

	child, ok := ss.m[fields[0]]
	if !ok {
		child = newStructSet()
		ss.m[fields[0]] = child
	}
	return child.hashConsInternal(fields[1:], typ)
}

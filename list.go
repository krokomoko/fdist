package fdist

type List struct {
	Start *Node
	End   *Node
}

type Node struct {
	Value *Class
	Prev  *Node
	Next  *Node
}

func (l *List) add(class *Class) {
	node := &Node{
		Value: class,
	}
	if l.End == nil {
		l.Start = node
		l.End = node
	} else {
		node.Prev = l.End
		l.End.Next = node
		l.End = l.End.Next
	}
}

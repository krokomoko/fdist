package fdist

type List struct {
	start *Node
	end   *Node
}

type Node struct {
	value *Class
	prev  *Node
	next  *Node
}

func (l *List) add(class *Class) {
	node := &Node{
		value: class,
	}
	if l.end == nil {
		l.start = node
		l.end = node
	} else {
		node.prev = l.end
		l.end.next = node
		l.end = l.end.next
	}
}

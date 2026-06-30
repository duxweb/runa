package filter

import "testing"

func TestFilterBuilders(t *testing.T) {
	eq := Eq[int]("status").Field("state")
	if eq.Name != "status" || eq.Target != "state" || eq.Operator != EqOp {
		t.Fatalf("eq = %#v", eq)
	}
	like := Like("name")
	if like.Target != "name" || like.Operator != LikeOp {
		t.Fatalf("like = %#v", like)
	}
	in := In[string]("role")
	if in.Operator != InOp {
		t.Fatalf("in = %#v", in)
	}
	between := Between[int]("price")
	if between.Operator != BetweenOp {
		t.Fatalf("between = %#v", between)
	}
	search := Search("keyword", "title", "body")
	if search.Operator != SearchOp || len(search.Meta["fields"].([]string)) != 2 {
		t.Fatalf("search = %#v", search)
	}
	sw := Switch[int]("status", map[int]int{1: 2})
	if sw.Operator != SwitchOp || sw.Meta["values"] == nil {
		t.Fatalf("switch = %#v", sw)
	}
}

package main

import "strconv"

type Question struct {
	Prompt string
	AKey   string
	AText  string
	BKey   string
	BText  string
}

var questions = []Question{
	{"The whole is more than the sum of its parts", "S", "is a saying that I never found very convincing", "N", "is a very important observation"},
	{"A small project", "J", "should be planned as well as a big one", "P", "allows to jump right in"},
	{"Abstract art", "S", "tends to annoy me", "N", "is often very interesting"},
	{"Arguments should be won", "T", "by the person with the better arguments", "F", "by the person fighting for the better cause"},
	{"Games that have no winner", "J", "I usually do not like", "P", "can be fun"},
	{"Going to the movies alone is", "E", "horrible", "I", "OK"},
	{"Helpless people", "T", "should be taught to help themselves", "F", "need help NOW"},
	{"I enjoy a walk most", "E", "together with a good friend", "I", "in the peace and quiet of nature"},
	{"I like teamwork best", "E", "in the beginning when work has to be talked over", "I", "in those times when I know what to do and can do it alone"},
	{"I often find emotional people", "T", "irritating", "F", "likable"},
	{"I prefer taxi drivers that", "E", "talk to me", "I", "drive silently"},
	{"I prefer to", "J", "plan my day even if it is difficult to foresee", "P", "wait and see"},
	{"I prefer", "I", "reading", "E", "games with others"},
	{"I respect people most for", "T", "what they achieve", "F", "the attitudes they have"},
	{"I tend to believe claims", "N", "when they are plausible to me", "S", "when I have concrete information that supports them"},
	{"I'd rather", "F", "fail than doing something wrong", "T", "bend a rule a little than fail"},
	{"If I were a super-popular star, I'd rather be", "E", "an actor", "I", "an author"},
	{"If a project gets behind schedule", "P", "one should adapt the schedule accordingly", "J", "it should make an effort to get back on schedule"},
	{"If two employees are having an argument, an ideal boss would", "P", "often let them solve it alone", "J", "usually step in and sort it out"},
	{"In my spare time", "E", "I like to be with people", "I", "I often prefer to be alone"},
	{"Laying off employees is", "F", "sad", "T", "sometimes necessary"},
	{"Making decisions", "P", "tends to be hard", "J", "tends to be easy"},
	{"On parties", "I", "I often stand alone", "E", "I tend to be in a crowd"},
	{"Rules are", "J", "generally helpful", "P", "often annoying"},
	{"Skilled rhetoric is", "F", "often a way of manipulating people", "T", "a tool for expressing oneself clearly and convincingly"},
	{"The reasons for doing something should be", "F", "morally valuable", "T", "logical and pragmatic"},
	{"To be a judge in an literature competition", "P", "I would tend to find straining and unsatisfying", "J", "I would find an interesting task"},
	{"To decide if a work of art is good", "S", "I need to know what are important aspects for this kind of art", "N", "I just see whether it appeals to me"},
	{"To understand the situation of a group of people, I rather rely on", "N", "descriptions of typical cases", "S", "survey results"},
	{"Usability tests are important because", "F", "low usability is bad for people", "T", "otherwise few people will use the product later"},
	{"What I enjoy most when I go to a conference is", "I", "the talks", "E", "the talking"},
	{"When 85-year-olds get an artificial hip that is", "F", "a wonderful victory of medicine over nature", "T", "a use of resources that ought to be weighed against alternatives"},
	{"When I create something", "S", "I tend to put together elements of existing things", "N", "I create it anew as a whole"},
	{"When I had a disappointment", "E", "I like it when friends help me get over it", "I", "I get over it best all by myself"},
	{"When I prepare a decision", "S", "I gather as much concrete information as I can", "N", "I also trust my intuition"},
	{"When I view a sports match of two foreign national teams", "P", "I often remain neutral", "J", "I prefer taking sides for one of them"},
	{"When a company rule appears not to fit a situation", "J", "I usually stick to it anyway. That is what rules are for.", "P", "I wonder whether I should make an exception"},
	{"When asked to comment on a complex plan or idea", "S", "I look for details that may be wrong", "N", "I tend to answer on a hunch"},
	{"When confronted with a new software application", "N", "I like to understand its basic concepts first", "S", "I prefer delving in and doing concrete things with it"},
	{"When confronted with a sketch of a great design of a building or machine", "N", "I am impressed and inspired", "S", "I'd rather see the final product"},
}

type TttResult struct {
	Result      string
	Type        string
	Temperament string
	EI          int
	SN          int
	TF          int
	JP          int
}

func evaluateAnswers(answers map[int]string) *TttResult {
	counts := map[string]int{"E": 0, "I": 0, "S": 0, "N": 0, "T": 0, "F": 0, "J": 0, "P": 0}
	for _, c := range answers {
		if _, ok := counts[c]; ok {
			counts[c]++
		}
	}
	if min(counts["E"]+counts["I"], counts["S"]+counts["N"], counts["T"]+counts["F"], counts["J"]+counts["P"]) < 5 {
		return nil
	}
	l1, d1, v1 := dim(counts, "E", "I", "I")
	l2, d2, v2 := dim(counts, "S", "N", "S")
	l3, d3, v3 := dim(counts, "T", "F", "T")
	l4, d4, v4 := dim(counts, "J", "P", "J")
	return &TttResult{
		Result:      l1 + "+" + strconv.Itoa(d1) + l2 + "+" + strconv.Itoa(d2) + l3 + "+" + strconv.Itoa(d3) + l4 + "+" + strconv.Itoa(d4),
		Type:        l1 + l2 + l3 + l4,
		Temperament: l2 + l4,
		EI:          v1,
		SN:          v2,
		TF:          v3,
		JP:          v4,
	}
}

func dim(counts map[string]int, a, b, tie string) (letter string, diff int, signed int) {
	av, bv := counts[a], counts[b]
	if av > bv {
		return a, av - bv, av - bv
	}
	if bv > av {
		return b, bv - av, -(bv - av)
	}
	return tie, 0, 0
}

func min(v ...int) int {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

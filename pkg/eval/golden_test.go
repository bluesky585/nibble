package eval

import (
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/store"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// goldenSet is the fixed collection the gate runs against: six short
// documents on distinct topics, three in English and three in Chinese,
// each a few sentences so the recursive chunker yields one to three
// chunks per document. The topics are deliberately far apart — sleep
// cycles, tea brewing, tide pools — so a relevant chunk has no excuse
// to lose to an unrelated one, and a drop in recall names a real
// regression rather than a hard question.
var goldenSet = Set{
	Docs: map[string]string{
		"sleep-en.txt": "Cats sleep twelve to sixteen hours a day, most of it in short naps rather than one long block. They are crepuscular, which means they are most active at dawn and dusk. Deep sleep comes in bursts of a few minutes, and a sleeping cat's ears keep tracking sounds while its body rests.",
		"tea-en.txt":   "Green tea is brewed with water around eighty degrees Celsius, cooler than black tea wants. Boiling water scalds the leaves and pulls out bitterness that masks the sweeter notes. Steeping lasts one to two minutes; leaving the leaves in longer turns the cup astringent.",
		"tide-en.txt":  "Tide pools form in the rocky zone between high and low tide, where the sea leaves behind water twice a day. The animals that live there — anemones, limpets, hermit crabs — tolerate huge swings in temperature and salinity. At low tide they seal themselves in; at high tide the pool rejoins the ocean.",
		"猫睡眠-zh.txt":   "猫每天睡十二到十六个小时，大多是短时间的小睡，而不是一整段长觉。猫是晨昏活动的动物，黎明和黄昏最精神。深睡每次只持续几分钟，睡觉时耳朵仍在追踪声音，身体却在休息。",
		"泡茶-zh.txt":    "绿茶要用大约八十摄氏度的水冲泡，比红茶需要的温度低。沸水会烫伤茶叶，泡出掩盖甘味的苦涩。冲泡一到两分钟即可；茶叶泡太久会让茶汤变得涩口。",
		"潮池-zh.txt":    "潮池形成于高潮线和低潮线之间的岩石区，海水每天两次把水留在这里。海葵、帽贝、寄居蟹都住在潮池里，忍受温度和盐度的剧烈变化。退潮时它们把自己封起来，涨潮时潮池重新汇入大海。",
	},
	Cases: []Case{
		{Query: "how many hours does a cat sleep each day", Relevant: "sleep-en.txt", Answers: []string{"Cats sleep twelve to sixteen hours"}},
		{Query: "when are cats most active", Relevant: "sleep-en.txt", Answers: []string{"crepuscular"}},
		{Query: "what water temperature for green tea", Relevant: "tea-en.txt", Answers: []string{"eighty degrees"}},
		{Query: "how long to steep green tea", Relevant: "tea-en.txt", Answers: []string{"Steeping lasts one to two minutes"}},
		{Query: "what animals live in a tide pool", Relevant: "tide-en.txt", Answers: []string{"anemones, limpets", "hermit crabs"}},
		{Query: "where do tide pools form", Relevant: "tide-en.txt", Answers: []string{"between high and low tide"}},
		{Query: "猫每天睡多久", Relevant: "猫睡眠-zh.txt", Answers: []string{"猫每天睡十二到十六个小时"}},
		{Query: "猫白天还是晚上活动", Relevant: "猫睡眠-zh.txt", Answers: []string{"黎明和黄昏最精神"}},
		{Query: "绿茶用什么水温冲泡", Relevant: "泡茶-zh.txt", Answers: []string{"八十摄氏度"}},
		{Query: "茶叶泡多久合适", Relevant: "泡茶-zh.txt", Answers: []string{"一到两分钟"}},
		{Query: "潮池里住着什么动物", Relevant: "潮池-zh.txt", Answers: []string{"海葵、帽贝、寄居蟹"}},
		{Query: "潮池是怎么形成的", Relevant: "潮池-zh.txt", Answers: []string{"高潮线和低潮线之间"}},
	},
}

// The gate: every scoring mode must place a relevant chunk in the top 3
// for every case in the golden set, with headroom — recall below the
// bar fails, and MRR is printed so a quiet slide is visible before it
// crosses. The bar sits where today's implementations comfortably land;
// it is a regression gate, not a leaderboard.
func TestGoldenRetrieval(t *testing.T) {
	t.Parallel()

	c, err := recursive.New(tokenizer.Character{}, 80, nil)
	if err != nil {
		t.Fatal(err)
	}
	emb := embed.Hashing{}

	for _, mode := range []string{"dense", "bm25", "hybrid"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			results, err := Run(c, goldenSet.Docs, goldenSet.Cases, emb, mode, 0.5, 3)
			if err != nil {
				t.Fatal(err)
			}
			recall := RecallAtK(results)
			mrr := MRR(results)
			t.Logf("%s: recall@3 = %.2f, MRR = %.2f", mode, recall, mrr)
			if recall < 0.9 {
				for _, r := range results {
					if !r.Hit {
						t.Errorf("miss: %q (want %s, top hit was %q)",
							r.Case.Query, r.Case.Relevant, firstLine(r.TopHit))
					}
				}
				t.Fatalf("%s recall@3 = %.2f, below the 0.9 gate", mode, recall)
			}
			if mrr < 0.75 {
				t.Errorf("%s MRR = %.2f, below the 0.75 gate — answers are landing late", mode, mrr)
			}
		})
	}
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	if len(s) > 60 {
		return s[:60]
	}
	return s
}

// Metrics behave on known inputs: perfect rankings score 1, a miss
// scores 0, and the means come out exact.
func TestMetrics(t *testing.T) {
	t.Parallel()

	perfect := []Result{{Hit: true, Rank: 1}, {Hit: true, Rank: 1}}
	if got := RecallAtK(perfect); got != 1 {
		t.Fatalf("RecallAtK(perfect) = %v", got)
	}
	if got := MRR(perfect); got != 1 {
		t.Fatalf("MRR(perfect) = %v", got)
	}

	late := []Result{{Hit: true, Rank: 2}, {Hit: false, Rank: 0}}
	if got := RecallAtK(late); got != 0.5 {
		t.Fatalf("RecallAtK(late) = %v", got)
	}
	if got := MRR(late); got != 0.25 {
		t.Fatalf("MRR(late) = %v, want 0.25 (1/2 + 0 over 2 cases)", got)
	}

	if got := RecallAtK(nil); got != 0 {
		t.Fatalf("RecallAtK(nil) = %v", got)
	}
}

// A relevant hit is only counted when it comes from the document the
// case names, so a lucky keyword match in another document cannot
// paper over a regression.
func TestJudgeRequiresRelevantSource(t *testing.T) {
	t.Parallel()

	right := store.Hit{Record: store.Record{Source: "right.txt",
		Chunk: mustChunk("the answer is here")}, Score: 1}
	wrongDoc := store.Hit{Record: store.Record{Source: "other.txt",
		Chunk: mustChunk("the answer is here")}, Score: 2}
	wrongText := store.Hit{Record: store.Record{Source: "right.txt",
		Chunk: mustChunk("something unrelated")}, Score: 3}

	c := Case{Relevant: "right.txt", Answers: []string{"the answer"}}
	if res := judge(c, []store.Hit{wrongDoc, right}); !res.Hit || res.Rank != 2 {
		t.Fatalf("a hit from another document must not count: %+v", res)
	}
	if res := judge(c, []store.Hit{wrongText, right}); !res.Hit || res.Rank != 2 {
		t.Fatalf("a hit from the right document but wrong text must not count: %+v", res)
	}
	if res := judge(c, []store.Hit{right}); !res.Hit || res.Rank != 1 || res.Score != 1 {
		t.Fatalf("the answering hit should rank and score: %+v", res)
	}
	if res := judge(c, []store.Hit{wrongText}); res.Hit {
		t.Fatalf("no matching hit must leave the case missed: %+v", res)
	}
}

func mustChunk(text string) chunk.Chunk {
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		panic(err)
	}
	return ch
}

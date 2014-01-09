// Package cluster performs hierarchical clustering of term vectors.
package cluster

import (
	"math"
	"runtime"
	"sync"
)

// Vec stores term vectors. Length is the euclidean norm and must be updated by
// calling Renorm() whenever the contents of the map M are changed.
type Vec struct {
	M      map[string]int
	Length float64
}

// NewVec initializes a term vector with a list of strings.
func NewVec(list []string) (v Vec) {
	v.M = make(map[string]int, len(list))
	for _, key := range list {
		v.M[key] += 1
	}
	v.Renorm()
	return
}

// Renorm updates the cached euclidean norm of a term vector.
func (a *Vec) Renorm() {
	var sum float64
	for _, val := range a.M {
		sum += float64(val * val)
	}
	a.Length = math.Sqrt(sum)
}

// Add adds two vectors. a.Add(b) means that a is modified to contain the result
// of the addition.
func (a *Vec) Add(b Vec) {
	for key := range b.M {
		a.M[key] += b.M[key]
	}
	a.Renorm()
}

// Dot returns the inner product of two vectors.
func (a Vec) Dot(b Vec) (sum float64) {
	if len(a.M) > len(b.M) {
		a, b = b, a
	}
	for key := range a.M {
		sum += float64(a.M[key] * b.M[key])
	}
	return
}

// Sim returns the cosine of the angle between two vectors, otherwise known as
// cosine similarity. The result will be between -1 and 1 (0 and 1 if all
// elements of the vector are nonnegative).
//
//  -1: absolutely opposed
//   0: independent
//   1: absolutely similar
//
// See http://en.wikipedia.org/wiki/Cosine_similarity.
func (a Vec) Sim(b Vec) float64 {
	if a.Length == 0 || b.Length == 0 {
		return 0
	}
	return a.Dot(b) / (a.Length * b.Length)
}

type simEntry struct {
	i, j int
	val  float64
}

// A Merge represents one level of a hierarchical clustering dendrogram.
//
// See: http://en.wikipedia.org/wiki/Dendrogram.
type Merge struct {
	Left, Right int
}

// Cluster returns a hierarchical clustering of the input dataset. The input
// dataset consists of a slice of term vectors which form the initial clusters.
// Clusters are merged successively until there is only one left and the order
// of the merges determines the dendrogram of the cluster. Cluster returns the
// sequence in which clusters were merged.
//
// See: http://en.wikipedia.org/wiki/Hierarchical_clustering.
func Cluster(vecs []Vec) []Merge {
	sims := make([]simEntry, (len(vecs)-1)*len(vecs)/2)

	queue := make(chan int, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup

	// Start worker goroutines.
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go func() {
			for n := range queue {
				sims[n].val = vecs[sims[n].i].Sim(vecs[sims[n].j])
				wg.Done()
			}
		}()
	}

	// Populate the initial similarity matrix.
	n := 0
	for i := range vecs {
		for j := 0; j < i; j++ {
			sims[n] = simEntry{i, j, 0}
			wg.Add(1)
			queue <- n
			n++
		}
	}
	wg.Wait()

	merges := make([]Merge, 0, len(vecs)-1)

	// In each round, the two most similar clusters are merged. The number of
	// clusters decreases by 1 each round.
	for round := 0; round < len(vecs)-1; round++ {
		// Find the two most similar clusters.
		maxsim := sims[0]
		for _, sim := range sims {
			if sim.val > maxsim.val {
				maxsim = sim
			}
		}

		// Merge cluster j into cluster i.
		vecs[maxsim.i].Add(vecs[maxsim.j])

		// Update the similarity matrix. Remove all entries which refer to
		// cluster j and recalculate all entries which refer to cluster i.
		n = 0
		for _, sim := range sims {
			switch {
			case sim.i == maxsim.j || sim.j == maxsim.j:
				continue
			case sim.i == maxsim.i || sim.j == maxsim.i:
				sims[n] = simEntry{sim.i, sim.j, 0}
				wg.Add(1)
				queue <- n
			default:
				sims[n] = sim
			}
			n++
		}
		wg.Wait()

		sims = sims[:len(sims)-len(vecs)+round+1]
		merges = append(merges, Merge{maxsim.i, maxsim.j})
	}
	close(queue)
	return merges
}

// Interpret returns a list of significant clusters given the sequence of
// merges returned by Cluster.
func Interpret(merges []Merge) [][]int {
	clusters := make([][]int, len(merges)+1)
	flagged := make([][]int, 0)
	for i := range clusters {
		clusters[i] = []int{i}
	}
	for _, merge := range merges {
		if len(clusters[merge.Left]) > 3 && len(clusters[merge.Right]) > 3 {
			if len(clusters[merge.Left]) < len(merges)/2 {
				flagged = append(flagged, clusters[merge.Left])
			}
			if len(clusters[merge.Right]) < len(merges)/2 {
				flagged = append(flagged, clusters[merge.Right])
			}
		}
		clusters[merge.Left] = append(clusters[merge.Left],
			clusters[merge.Right]...)
	}
	return flagged
}

package main

import (
	"strings"
)

// flatten picks the index of each element in a 2D slice to create a 1D slice. For example, if `flatten` is called with
// an index of 1, every element of the `slice` that is passed into it will be inserted into the result slice by the
// index `[1]`
func flatten(slice [][]string, index int) []string {
	result := make([]string, len(slice))
	for i, el := range slice {
		result[i] = el[index]
	}
	return result
}

// unique removes all duplicate values in the given slice, maintaining the order the values are seen in the slice
func unique(slice []string) []string {
	result := make([]string, 0, len(slice))
	m := make(map[string]bool)
	for _, text := range slice {
		if m[text] {
			continue
		}
		m[text] = true
		result = append(result, text)
	}
	return result
}

// createParams returns a string of form "?, ?, ?" where each ? correlates to an entry in the given slice. This is to be
// used for variable length SELECT ... col IN (?, ?, ?) in prepared statements.
func createParams(slice []string) string {
	temp := make([]string, len(slice))
	for i := range slice {
		temp[i] = "?"
	}
	return strings.Join(temp, ", ")
}

// generify transforms a []string slice into a []interface{}. []string and []interface{} aren't represented the same
// in memory, so it's not valid to simply cast from one to the other
func generify(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, e := range slice {
		result[i] = e
	}
	return result
}

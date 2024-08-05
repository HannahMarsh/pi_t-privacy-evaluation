package utils

import (
	"cmp"
	"context"
	"fmt"
	"github.com/HannahMarsh/PrettyLogger"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

func ForEach[T any](items []T, f func(T)) {
	for _, item := range items {
		f(item)
	}
}

func FilterMap[K comparable, V any](m map[K]V, condition func(K, V) bool) map[K]V {
	filteredMap := make(map[K]V)
	for k, v := range m {
		if condition(k, v) {
			filteredMap[k] = v
		}
	}
	return filteredMap
}

func Remove[T any](items []T, condition func(T) bool) []T {
	filteredItems := make([]T, 0)
	for _, item := range items {
		if !condition(item) {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems
}

func RemoveElement[T comparable](items []T, element T) []T {
	return Remove(items, func(e T) bool {
		return e == element
	})
}

func HasUniqueElements[T comparable](items []T) bool {
	seen := make(map[T]bool)
	for _, item := range items {
		if seen[item] {
			return false
		}
		seen[item] = true
	}
	return true
}

func InsertAtIndex[T any](items []T, index int, value T) []T {
	if index == 0 {
		return append([]T{value}, items...)
	}
	items2 := Copy(items)
	if index == len(items2) {
		return append(items2, value)
	}
	temp := append(Copy(items2[:index]), value)
	return append(temp, items2[index:]...)
}

func RemoveIndex[T any](items []T, index int) []T {
	if index == 0 {
		return items[1:]
	}
	if index == len(items)-1 {
		return items[:len(items)-1]
	}
	return append(items[:index], items[index+1:]...)
}

func GetLast[T any](items []T) T {
	return items[len(items)-1]
}

func GetFirst[T any](items []T) T {
	return items[0]
}

func GetSecondFromLast[T any](items []T) T {
	return items[len(items)-2]
}

func MaxOver[T cmp.Ordered](items []T) T {
	maxValue := items[0]
	for _, item := range items {
		if item > maxValue {
			maxValue = item
		}
	}
	return maxValue
}

func MinOver[T cmp.Ordered](items []T) T {
	minValue := items[0]
	for _, item := range items {
		if item < minValue {
			minValue = item
		}
	}
	return minValue
}

func RemoveDuplicates[T comparable](items []T) []T {
	uniqueItems := make([]T, 0)
	seen := make(map[T]bool)
	for _, item := range items {
		if !seen[item] {
			uniqueItems = append(uniqueItems, item)
			seen[item] = true
		}
	}
	return uniqueItems
}

func Filter[V any](values []V, condition func(V) bool) []V {
	filteredValues := make([]V, 0)
	for _, v := range values {
		if condition(v) {
			filteredValues = append(filteredValues, v)
		}
	}
	return filteredValues
}

func Mean[T Number](values []T) float64 {
	sum := Sum(values)
	return toFloat64(sum) / float64(len(values))
}

func CompareArrays[T comparable](a, b []T) (bool, int) {
	if a == nil && b == nil {
		return true, -1
	}
	if a == nil || b == nil {
		return false, -1
	}
	if len(a) != len(b) {
		return false, -1
	}
	for i := range a {
		if a[i] != b[i] {
			return false, i
		}
	}
	return true, -1
}

type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | float32 | float64 | complex64 | complex128
}

func toFloat64[T Number](n T) float64 {
	switch v := any(n).(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case complex64:
		return float64(real(v)) // Considering only the real part
	case complex128:
		return real(v) // Considering only the real part
	default:
		panic(fmt.Sprintf("unsupported type: %T", v))
	}
}

func Sum[T Number](values []T) T {
	var sum T
	for _, v := range values {
		sum += v
	}
	return sum
}

func MaxValue(values []int) int {
	m := values[0]
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

func GetValues[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0)
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func MapToMap[K comparable, V any, O any](m map[K]V, f func(K, V) O) map[K]O {
	result := make(map[K]O)
	for k, v := range m {
		result[k] = f(k, v)
	}
	return result
}

func MapToArray[K comparable, V any, O any](m map[K]V, f func(K, V) O) []O {
	result := make([]O, 0)
	for k, v := range m {
		result = append(result, f(k, v))
	}
	return result
}

func MapToPointerArray[K comparable, V any, O any](m map[K]V, f func(K, V) *O) []*O {
	result := make([]*O, 0)
	for k, v := range m {
		if r := f(k, v); r != nil {
			result = append(result, r)
		}
	}
	return result
}

func FillArray[T any](value T, numElements int) []T {
	if numElements <= 0 {
		return []T{}
	}
	values := make([]T, numElements)
	for i := 0; i < numElements; i++ {
		values[i] = value
	}
	return values
}

func Shuffle[T any](items []T) {
	for i := len(items) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		items[i], items[j] = items[j], items[i]
	}
}

func GetShuffledCopy[T any](items []T) []T {
	result := Copy(items)
	Shuffle(result)
	return result
}

func GetKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func Map[T any, O any](items []T, f func(T) O) []O {
	result := make([]O, len(items))
	for i, item := range items {
		result[i] = f(item)
	}
	return result
}

func MapEntries[K comparable, V any, O any](m map[K]V, f func(K, V) O) []O {
	result := make([]O, 0, len(m))
	for k, v := range m {
		result = append(result, f(k, v))
	}
	return result
}

func Contains[T any](items []T, f func(T) bool) bool {
	for _, item := range items {
		if f(item) {
			return true
		}
	}
	return false
}

func DoesNotContain[T any](items []T, f func(T) bool) bool {
	return !Contains(items, f)
}

func Find[T any](items []T, f func(T) bool) *T {
	for _, item := range items {
		if f(item) {
			return &item
		}
	}
	return nil
}

func FindPointer[T any](items []*T, f func(*T) bool) *T {
	for _, item := range items {
		if f(item) {
			return item
		}
	}
	return nil
}

func FindInMap[K comparable, V any](m map[K]V, f func(K, V) bool, defaultKey K, defaultValue V) (K, V, bool) {
	for k, v := range m {
		if f(k, v) {
			return k, v, true
		}
	}
	return defaultKey, defaultValue, false
}

func FindKey[K comparable, V any](m map[K]V, f func(K, V) bool, defaultValue K) (K, bool) {
	for k, v := range m {
		if f(k, v) {
			return k, true
		}
	}
	return defaultValue, false
}

func FindValue[K comparable, V any](m map[K]V, f func(K, V) bool, defaultValue V) (V, bool) {
	for k, v := range m {
		if f(k, v) {
			return v, true
		}
	}
	return defaultValue, false
}

func DoesMapContain[K comparable, V any](m map[K]V, f func(K, V) bool) bool {
	for k, v := range m {
		if f(k, v) {
			return true
		}
	}
	return false
}

func DoesMapNotContain[K comparable, V any](m map[K]V, f func(K, V) bool) bool {
	return !DoesMapContain(m, f)
}

func FindIndex[T any](items []T, f func(T) bool) int {
	for i, item := range items {
		if f(item) {
			return i
		}
	}
	return -1
}

func FindLastIndex[T any](items []T, f func(T) bool) int {
	for i := len(items) - 1; i >= 0; i-- {
		if f(items[i]) {
			return i
		}
	}
	return -1
}

func Copy[T any](items []T) []T {
	result := make([]T, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func Swap[T any](items []T, i, j int) {
	items[i], items[j] = items[j], items[i]
}

func Flatten[T any](items [][]T) []T {
	var result []T
	for _, item := range items {
		result = append(result, item...)
	}
	return result
}

func FlatMap[T any, O any](items []T, f func(T) []O) []O {
	var result []O
	for _, item := range items {
		result = append(result, f(item)...)
	}
	return result
}

func Fold[T any, O any](items []T, initial O, f func(O, T) O) O {
	result := initial
	for _, item := range items {
		result = f(result, item)
	}
	return result
}

func Apply[T any](items []T, f func(T)) {
	for _, item := range items {
		f(item)
	}
}

func Unless[T any](items []T, f func(T) bool) bool {
	for _, item := range items {
		if !f(item) {
			return false
		}
	}
	return true
}

func MapParallel[T any, O any](items []T, f func(T) (O, error)) ([]O, error) {
	var wg sync.WaitGroup
	wg.Add(len(items))
	results := make([]O, len(items))
	errs := make([]error, len(items))
	for i, item := range items {
		go func(i int, item T) {
			defer wg.Done()
			results[i], errs[i] = f(item)
		}(i, item)
	}
	wg.Wait()
	// Aggregate errors
	var firstError error
	for _, err := range errs {
		if err != nil {
			if firstError == nil {
				firstError = err
			} else {
				firstError = PrettyLogger.WrapError(firstError, "MapParallel")
			}
		}
	}
	if firstError != nil {
		return nil, firstError
	}
	return results, nil
}

func FlatMapParallel[T any, O any](items []T, f func(T) ([]O, error)) ([]O, error) {
	if result, err := MapParallel(items, f); err != nil {
		return nil, err
	} else {
		return Flatten(result), nil
	}
}

func Reverse[T any](items []T) []T {
	result := make([]T, len(items))
	for i, item := range items {
		result[len(items)-1-i] = item
	}
	return result
}

func ParallelFind[T any](items []T, f func(T) bool) *T {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure all paths cancel the context to avoid a context leak

	var found atomic.Value
	found.Store((*T)(nil)) // Initialize with nil

	var wg sync.WaitGroup
	numProcs := runtime.NumCPU() // Number of logical CPUs
	segmentSize := (len(items) + numProcs - 1) / numProcs

	for i := 0; i < len(items); i += segmentSize {
		end := i + segmentSize
		if end > len(items) {
			end = len(items)
		}

		wg.Add(1)
		go func(segment []T) {
			defer wg.Done()
			for _, item := range segment {
				select {
				case <-ctx.Done():
					return // Exit if context is cancelled
				default:
					if f(item) {
						found.Store(&item)
						cancel() // Cancel other goroutines
						return
					}
				}
			}
		}(items[i:end])
	}

	wg.Wait()
	result, _ := found.Load().(*T)
	return result
}

func Sort[T any](items []T, less func(T, T) bool) {
	sort.Slice(items, func(i, j int) bool {
		return less(items[i], items[j])
	})
	//// Define the Lesswap function required by sorty
	//lesswap := func(i, k, r, s int) bool {
	//	if less(items[i], items[k]) {
	//		if r != s {
	//			items[r], items[s] = items[s], items[r]
	//		}
	//		return true
	//	}
	//	return false
	//}
	//
	//// Call sorty.Sort with the length of the items and the lesswap function
	//sorty.Sort(len(items), lesswap)
}

func SortOrdered[T cmp.Ordered](items []T) {
	Sort(items, func(a, b T) bool {
		return a < b
	})
}

func ParallelContains[T any](items []T, f func(T) bool) bool {
	return ParallelFind(items, f) != nil
}

func FindLast[T any](items []T, f func(T) bool) *T {
	for i := len(items) - 1; i >= 0; i-- {
		if f(items[i]) {
			return &items[i]
		}
	}
	return nil
}

func Count[T comparable](items []T, value T) int {
	count := 0
	for _, item := range items {
		if item == value {
			count++
		}
	}
	return count
}

func CountAny[T any](items []T, f func(T) bool) int {
	count := 0
	for _, item := range items {
		if f(item) {
			count++
		}
	}
	return count
}

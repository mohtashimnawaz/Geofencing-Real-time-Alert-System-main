package main

import "testing"

func TestPointInPolygon(t *testing.T) {
	p := [][]float64{{0, 0}, {0, 10}, {10, 10}, {10, 0}, {0, 0}}
	if !pointInPolygon(5, 5, p) || pointInPolygon(20, 5, p) {
		t.Fatal("point-in-polygon result incorrect")
	}
}

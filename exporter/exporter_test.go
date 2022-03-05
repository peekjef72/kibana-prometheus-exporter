package exporter

import (
	"testing"
)

func TestNewExporterWithoutNamespace(t *testing.T) {
	colls := make([]*KibanaCollector, 1)
	colls = append(colls, &KibanaCollector{})
	_, err := NewExporter("", colls, false, nil)
	if err == nil {
		t.Errorf("expected error when invalid namespace was provided")
	}
}

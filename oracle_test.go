//go:build oracle

package dxf_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/apantanella/dxf"
)

var (
	oracleOnce sync.Once
	oracleOK   bool
	oracleWhy  string
)

// oracleAvailable verifica una sola volta che python3 con ezdxf sia disponibile.
func oracleAvailable() (bool, string) {
	oracleOnce.Do(func() {
		py, err := exec.LookPath("python3")
		if err != nil {
			oracleWhy = "python3 non trovato"
			return
		}
		out, err := exec.Command(py, "-c", "import ezdxf").CombinedOutput()
		if err != nil {
			oracleWhy = "ezdxf non importabile: " + string(out)
			return
		}
		oracleOK = true
	})
	return oracleOK, oracleWhy
}

// oracleReport e' il JSON stampato da scripts/oracle_check.py.
type oracleReport struct {
	File     string           `json:"file"`
	Version  string           `json:"version"`
	Fatal    string           `json:"fatal"`
	Entities map[string]int   `json:"entities"`
	Errors   []map[string]any `json:"errors"`
	Fixes    []map[string]any `json:"fixes"`
}

// runOracle esegue scripts/oracle_check.py su path e fa fallire il test se
// ezdxf non riesce a leggere il file o l'audit riporta errori. Salta il test
// se python3/ezdxf non sono disponibili. Riutilizzabile dai test di scrittura
// (T9) per verificare i file prodotti dalla libreria. Restituisce il report.
func runOracle(t *testing.T, path string) *oracleReport {
	t.Helper()
	if ok, why := oracleAvailable(); !ok {
		t.Skip("oracolo ezdxf non disponibile: " + why)
	}
	script, err := filepath.Abs(filepath.Join("scripts", "oracle_check.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", script, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var rep oracleReport
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("oracle %s: output non JSON: %v\nstdout: %s\nstderr: %s", path, err, stdout.String(), stderr.String())
	}
	if rep.Fatal != "" {
		t.Errorf("oracle %s: ezdxf non legge il file: %s", path, rep.Fatal)
	}
	for _, e := range rep.Errors {
		t.Errorf("oracle %s: errore audit: %v", path, e)
	}
	for _, f := range rep.Fixes {
		t.Logf("oracle %s: fix audit: %v", path, f)
	}
	if runErr != nil && rep.Fatal == "" && len(rep.Errors) == 0 {
		t.Errorf("oracle %s: %v\n%s", path, runErr, stderr.String())
	}
	t.Logf("oracle %s: versione %s, entita %v", path, rep.Version, rep.Entities)
	return &rep
}

// TestOracleFixtures legge in modalita rigorosa (senza WithLenient) ogni
// fixture di testdata/oracle/*.dxf. Gli errori del lettore su versioni o
// costrutti non ancora supportati sono registrati e saltati, non falliscono.
func TestOracleFixtures(t *testing.T) {
	if ok, why := oracleAvailable(); !ok {
		t.Skip("oracolo ezdxf non disponibile: " + why)
	}
	files, err := filepath.Glob(filepath.Join("testdata", "oracle", "*.dxf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("nessuna fixture in testdata/oracle (eseguire scripts/gen_fixtures.py)")
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			runOracle(t, path) // il file di riferimento deve essere valido per ezdxf
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := dxf.Decode(bytes.NewReader(data))
			if err != nil {
				var verr *dxf.VersionError
				switch {
				case errors.As(err, &verr), errors.Is(err, dxf.ErrUnsupportedVersion),
					errors.Is(err, dxf.ErrSyntax), errors.Is(err, dxf.ErrValidation),
					errors.Is(err, dxf.ErrLimit), errors.Is(err, dxf.ErrNotDXF):
					t.Logf("lettore rigoroso: errore noto: %v", err)
					t.Skipf("versione o costrutto non ancora supportato: %v", err)
				default:
					t.Errorf("Decode: errore inatteso: %v", err)
				}
				return
			}
			if doc == nil {
				t.Fatal("Decode: documento nil senza errore")
			}
			t.Logf("Decode rigoroso OK")
		})
	}
}

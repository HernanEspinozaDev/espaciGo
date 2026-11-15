# Evidencia — VI-04, primitivas del escáner estructural de PDF y validación de buffers en inspection

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit base `7d6d7c67425d0c72c918efeb10344b584d436df7`.
- **Entorno:** Local aislado Linux x86_64, Go 1.24 toolchain, PostgreSQL persistente ausente y no invocado.
- **Gate local:** Unitario/caracterización test-only con fixtures sintéticos en memoria. Sin SQL/DDL, sin acceso DB persistente ni Cloud Console / Cloud Storage.

## Verificaciones cubiertas

1. **Límites de decodificación y presupuesto (`pdfDecodeLimits`):**
   - Retorno fail-closed con `ErrRejected` ante presupuesto total `<= 0` (probado con `0` y `-10`).
   - Retorno íntegro del presupuesto cuando se encuentra dentro del rango válido.
   - Acotamiento exacto a `maxPDFDecodedStream` ante presupuestos que exceden el techo superior configurado.
   - Cobertura de statements: **100,0%**.

2. **Clasificación léxica de delimitadores y espacios en blanco (`isPDFWhitespace`, `isPDFDelimiter`):**
   - Verificación exhaustiva de bytes de espacio en blanco según estándar PDF (0x00, '\t', '\n', '\f', '\r', ' ').
   - Verificación de caracteres delimitadores (`(`, `)`, `<`, `>`, `[`, `]`, `{`, `}`, `/`, `%`).
   - Verificación de que caracteres alfanuméricos y símbolos regulares no se confunden con delimitadores o espacios.
   - Cobertura de statements: **100,0%**.

3. **Omisión de espacios y comentarios (`skipPDFSpaceAndComments`):**
   - Desplazamiento correcto a través de secuencias compuestas de espacios, tabulaciones, saltos de línea y comentarios iniciados con `%`.
   - Cobertura de statements: **100,0%**.

4. **Análisis de enteros y referencias indirectas (`scanPDFInteger`, `isPDFIndirectReferenceAt`):**
   - Extracción de enteros y reconocimiento de formato `N G R` (objeto, generación, token 'R').
   - Rechazo fail-closed de referencias indirectas ante generaciones no enteras, números de objeto no enteros o ausencia del token 'R'.
   - Cobertura de statements: **100,0%**.

5. **Omisión de cadenas literales PDF (`skipPDFLiteralString`):**
   - Soporte de paréntesis anidados y secuencias de escape `\(`, `\)`.
   - Detección de cadenas no cerradas retornando `false`.
   - Rechazo ante tokens que no inician con `(` o cuando el offset inicial supera la longitud del buffer.
   - Manejo de caracteres de escape al final del buffer (EOF).
   - Cobertura de statements: **92,3%**.

6. **Lectura y decodificación de nombres PDF (`readPDFName`, `decodePDFNameEscape`):**
   - Decodificación de escapes hexadecimales `#XX` con caracteres numéricos, mayúsculas ('A'-'F') y minúsculas ('a'-'f').
   - Manejo de escapes inválidos (hexadecimal no válido) y secuencias `#` truncadas al final del buffer sin pánico.
   - Extracción de nombres delimitados al final de buffer sin delimitador posterior.
   - Cobertura de statements: `readPDFName` al **100,0%**, `decodePDFNameEscape` al **100,0%**.

7. **Reconocimiento de decodificadores y detección de filtros indirectos (`isPDFDecoderName`, `hasIndirectPDFFilter`):**
   - Verificación de nombres estándar y abreviaturas canónicas (FlateDecode/Fl, ASCII85Decode/A85, DCTDecode/DCT, etc.).
   - Rechazo de nombres ajenos o filtros vacíos.
   - Detección en `hasIndirectPDFFilter` sobre las tipologías sintácticas evaluadas: referencias directas, indirectas individuales `12 0 R`, arreglos directos `[/FlateDecode /ASCII85Decode]`, arreglos con referencias indirectas `[/ASCII85Decode 15 0 R]`, comentarios en arreglos, cadenas literales, diccionarios, cadenas hexadecimales, arreglos anidados y arreglos no cerrados.
   - Cobertura de statements: `isPDFDecoderName` al **100,0%**, `hasIndirectPDFFilter` al **100,0%**.

8. **Conteo de etapas de decodificación y marcadores de flujo (`countPDFDecoderStages`, `countPDFXRefReferences`, `countPDFStreamMarkers`, `countPDFNameOccurrences`):**
   - Rechazo con `ErrRejected` ante filtros indirectos en `countPDFDecoderStages`.
   - Conteo exacto de etapas legítimas en filtros compuestos.
   - Conteo de ocurrencias de `startxref`, `Prev` y `XRefStm`.
   - Conteo conservador por coincidencia de subcadena de marcadores `stream` delimitados (que por diseño intencional también computa la subcadena en terminadores `endstream`).
   - Cobertura de statements: `countPDFXRefReferences` al **100,0%**, `countPDFStreamMarkers` al **100,0%**, `countPDFDecoderStages` al **92,9%**, `countPDFNameOccurrences` al **91,7%**.

9. **Límites de salida acotada y validación estructural (`boundedOutput`, `validatePDFStructureWithBudget`):**
   - `boundedOutput.Write` acepta escrituras dentro del límite y exactamente al límite `maxParserOutputBytes`, preserva el buffer intacto ante escrituras que desborden y retorna `errParserOutputLimit`.
   - `validatePDFStructureWithBudget` aplica guards fail-closed ante páginas esperadas fuera de rango (`< 1` o `> maxPDFPages`), datos vacíos, `maxPixels <= 0` o `decodedBudget <= 0`.
   - Cobertura de statements: `boundedOutput.Write` al **100,0%**, `boundedOutput.String` al **100,0%**, `validatePDFStructure` al **100,0%**, `validatePDFStructureWithBudget` al **87,5%**.

## Comprobación local ejecutada

```bash
$ go test -race -v -run 'Test(PDF|ScanPDF|SkipPDF|ReadPDF|IsPDF|HasIndirectPDF|CountPDF|BoundedOutput|ValidatePDFStructureWithBudget)' ./internal/documentverification/inspection
PASS
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection   1.352s
```

```bash
$ go test -coverprofile=/tmp/cov_pdf_struct.out ./internal/documentverification/inspection
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection   1.132s   coverage: 84.4% of statements
```

```bash
$ go test -race -count=1 ./internal/documentverification/...
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification              1.050s
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection   15.192s
ok      github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http 1.481s
```

## Cobertura consolidada

- `internal/documentverification/inspection`: **84,4%** (incremento desde 78,4%).
- Cobertura consolidada del subsistema (`internal/documentverification/...`): **91,4%** de statements.
- Todos los tests unitarios ejecutados en memoria con fixtures sintéticos sin dependencias de red, base de datos ni storage real.

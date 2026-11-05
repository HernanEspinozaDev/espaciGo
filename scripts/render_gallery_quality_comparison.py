#!/usr/bin/env python3
"""Render the measured, authorized corpus as a portable offline comparison.

Does not change the production encoder or declare human visual acceptance.
Run the opt-in Go experiment first; all images are embedded as data URLs.
"""
import argparse
import base64
import html
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / "planning/frontend_next/evidence"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if not args.corpus.is_absolute() or not args.output.is_absolute():
        raise SystemExit("Explicit absolute corpus and output paths required")
    if args.output.exists():
        raise SystemExit("Preserve the existing comparison; select a fresh output file")
    results = (args.corpus / "results").resolve(strict=True)
    report = json.loads((EVIDENCE / "gallery-authorized-measurements-20261105.json").read_text())
    manifest = json.loads((EVIDENCE / "gallery-authorized-corpus.json").read_text())
    alternatives, production = report["alternatives"], report["production"]
    if len(alternatives) != 72 or len(production) != 12:
        raise SystemExit("Incomplete measured corpus; do not render a partial comparison")
    panels = []
    for case, title in [
        ("exterior_texture", "Textura sintética JPEG de estrés"),
        ("webp_texture", "Textura sintética WebP de estrés"),
        ("living_room", "Foto común: sala (CC0)"),
        ("white_office", "Foto común: oficina (CC0)"),
        ("wood_texture", "Textura fotográfica: madera (CC0)"),
        ("transparent_plan", "Transparencia sintética: alfa96 real"),
    ]:
        baseline = next(r for r in production if r["case"] == case and r["kind"] == "display")
        if case.endswith("texture") and case != "wood_texture":
            fitting = [r for r in alternatives if r["case"] == case and r["bytes"] <= 614400]
            maximum_resolution = max(fitting, key=lambda r: (r["max_side"], r["quality"]))
            maximum_quality = max(fitting, key=lambda r: (r["quality"], r["max_side"]))
            options = [("Mayor resolución que cumple peso", maximum_resolution), ("Mayor Q que cumple peso", maximum_quality)]
        else:
            candidate = next(r for r in alternatives if r["case"] == case and r["max_side"] == 1600 and r["quality"] == 70)
            options = [("Comparación Q70; no propuesta automática", candidate)]
        cards = [card(results, "Referencia decodificada a1600px", case + "_1600_reference.png", case + "_1600_reference_crop.png", "Referencia sin compresión WebP adicional; no es el archivo original a tamaño completo.")]
        cards.append(metric_card(results, "Producción anterior a la decisión:1600/Q80", baseline, production=True))
        for label, row in options:
            cards.append(metric_card(results, label, row))
        panels.append("<section><h2>" + html.escape(title) + "</h2><div class='grid'>" + "".join(cards) + "</div></section>")
    table = "".join(
        "<tr><td>" + html.escape(r["case"]) + "</td><td>" + html.escape(r["kind"]) +
        f"</td><td>{r['width']}×{r['height']}</td><td>{r['bytes']:,}</td><td>" +
        ("Cumple" if r["bytes"] <= (614400 if r["kind"] == "display" else 102400) else "NO cumple") +
        f"</td><td>{r['alpha_error_max']}</td></tr>" for r in production
    )
    credits = "".join("<li>" + html.escape(s["title"] + " — " + s["author"] + " — CC0. " + s["pageURL"]) + "</li>" for s in manifest["sources"])
    body = """<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'">
<title>EspaciGo · Comparación medida de calidad de galería</title>
<style>html{color-scheme:light}body{font:16px/1.5 system-ui,sans-serif;margin:24px;color:#172033;background:#f8fafc}h1{font-size:28px}h2{margin:32px 0 12px;font-size:22px}.notice{padding:16px;border-left:4px solid #0284c7;background:#e0f2fe}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(290px,1fr));gap:16px}.card{padding:14px;min-width:0;background:white;border:1px solid #cbd5e1;border-radius:8px}.card h3{font-size:17px;margin:0 0 12px}.preview{height:230px;background:repeating-conic-gradient(#ddd 0% 25%,white 0% 50%) 50%/20px 20px}.preview img{display:block;width:100%;height:100%;object-fit:contain}.zoom{overflow:auto;height:240px;background:repeating-conic-gradient(#ddd 0% 25%,white 0% 50%) 50%/20px 20px}.zoom img{display:block;max-width:none;width:auto;height:auto}.stats{min-height:110px;font-size:14px}.tag{font-weight:700}.fail{color:#b91c1c}.pass{color:#166534}table{border-collapse:collapse;background:white}td,th{text-align:left;border:1px solid #cbd5e1;padding:7px}li{overflow-wrap:anywhere}.table{overflow:auto}small{color:#475569}section{padding-bottom:18px;border-bottom:1px solid #cbd5e1}</style>
<h1>Comparación medida de calidad de galería</h1>
<div class="notice"><strong>Comparación histórica previa a la decisión humana del piso1280/Q75.</strong><br>Metas sin cambio: display≤614.400bytes y thumbnail≤102.400bytes. Las tres fotos CC0 y la transparencia cumplen; los dos displays de ruido sintético no cumplían a1600/Q80. Esta hoja conserva la comparación que fundamentó la elección; los resultados posteriores figuran por separado en gallery-authorized-policy-after-20261105.json. No constituye aceptación global del producto.</div>
<p>84 mediciones reales: 72 alternativas y12 variantes de <code>Convert</code>. La referencia es la entrada decodificada y redimensionada independientemente. PSNR y SSIM no incluyen la pérdida espacial al bajar resolución; SSIM es global de luminancia, no local/MS-SSIM. Los dos casos de estrés son ruido sintético y no representan una fotografía habitual.</p>
<p>La vista completa se ajusta a la columna. Debajo, el recorte se muestra al100%: puedes desplazarlo sin escalado. No comparar solo los números de PSNR entre resoluciones distintas.</p>
""" + "".join(panels) + "<h2>Producción actual: resultados completos</h2><div class='table'><table><tr><th>Caso</th><th>Variante</th><th>Píxeles</th><th>Bytes</th><th>Meta de peso</th><th>Error alfa máximo</th></tr>" + table + "</table></div><h2>Fuentes autorizadas</h2><p>Autores y licencia contrastados con extmetadata de la API oficial de Wikimedia Commons; SHA de cada original verificado contra el manifest versionado. Sin datos privados ni solicitudes de red desde este archivo.</p><ul>" + credits + "</ul><p><small>Ensayo local sintético. Ningún resultado autoriza main, despliegue o F7.</small></p></html>"
    args.output.write_text(body, encoding="utf-8")
    args.output.chmod(0o600)
    print("OFFLINE_QUALITY_COMPARISON_RENDERED", len(panels), "CASES", len(body.encode()), "BYTES")


def embedded(results, name):
    path = (results / name).resolve(strict=True)
    if not path.is_relative_to(results) or path.suffix not in (".png", ".webp"):
        raise ValueError("Invalid measured image path")
    mime = "image/png" if path.suffix == ".png" else "image/webp"
    return "data:" + mime + ";base64," + base64.b64encode(path.read_bytes()).decode("ascii")


def card(results, title, image, crop, stats):
    return "<article class='card'><h3>" + html.escape(title) + "</h3><div class='preview'><img alt='Vista completa' src='" + embedded(results, image) + "'></div><p class='stats'>" + stats + "</p><p>Recorte al100%, sin escalado</p><div class='zoom'><img alt='Recorte central a tamaño nativo' src='" + embedded(results, crop) + "'></div></article>"


def metric_card(results, label, row, production=False):
    case = row["case"]
    stem = case + "_production_display" if production else f"{case}_{row['max_side']}_q{row['quality']}"
    if (results / (stem + ".webp")).stat().st_size != row["bytes"]:
        raise ValueError("Derivative does not match measured byte count")
    passes = row["bytes"] <= 614400
    stats = f"{row['width']}×{row['height']}px · Q{row['quality']} · <strong>{row['bytes']:,}bytes</strong><br>PSNR {row['psnr_rgb_db']:.2f}dB · SSIM global {row['ssim_global_luminance']:.6f}<br>Error alfa máximo: {row['alpha_error_max']}<br><span class='tag {'pass' if passes else 'fail'}'>{'Cumple peso' if passes else 'NO cumple peso'}</span>"
    return card(results, label, stem + ".webp", stem + "_crop.png", stats)


if __name__ == "__main__":
    main()

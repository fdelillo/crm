#!/usr/bin/env python3
"""Valida el atlas y sus permalinks con objetos Git locales (sin descargar fuentes).
Uso: python3 docs/diagramas/validar.py --repo . --self-check /ruta/diagram-design/scripts/self_check.py
La adaptación del self_check localiza los literales del controlador y permite solo los hipervínculos GitHub ya validados; no permite recursos remotos.
"""
from pathlib import Path
from html.parser import HTMLParser
from urllib.parse import urlsplit, unquote
import argparse, importlib.util, json, re, subprocess, sys

TRANSLATIONS = {
    "'Ready'": "'Listo'", "'Complete'": "'Completo'",
    "`Step ${step} of ${count}: ${labels[step] || 'Completo'}`": "`Paso ${step} de ${count}: ${labels[step] || 'Completo'}`",
    '`Paused at step ${step} of ${count}`': '`Pausado en el paso ${step} de ${count}`',
    '`Complete · step ${count} of ${count}`': '`Completo · paso ${count} de ${count}`',
    'Reduced motion · complete static frame · playback controls unavailable': 'Movimiento reducido · diagrama completo estático · controles de reproducción no disponibles',
    'Static frame · complete diagram · playback controls unavailable': 'Vista estática · diagrama completo · controles de reproducción no disponibles',
    '`Test frame · step ${step} of ${count} · playback controls unavailable`': '`Vista de prueba · paso ${step} de ${count} · controles de reproducción no disponibles`',
    '`Ready · step ${step} of ${count}`': '`Listo · paso ${step} de ${count}`',
    '`Ready · step 0 of ${count}`': '`Listo · paso 0 de ${count}`',
}

class Links(HTMLParser):
    def __init__(self):
        super().__init__(); self.hrefs=[]; self.ids=set(); self.resources=[]
    def handle_starttag(self, tag, attrs):
        a=dict(attrs)
        if 'id' in a:self.ids.add(a['id'])
        if tag=='a' and a.get('href'):self.hrefs.append(a['href'])
        if tag in ('script','img','iframe','link'):
            self.resources.extend(a[k] for k in ('src','href') if a.get(k))

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo',type=Path,required=True)
    parser.add_argument('--self-check',type=Path)
    args=parser.parse_args();root=Path(__file__).resolve().parent
    manifest=json.loads((root/'manifest.json').read_text());sha=manifest['commit']
    assert re.fullmatch('[0-9a-f]{40}',sha)
    issues=[];urls=set();local_links=0;figures=0;interactive=0;diagram_pages=[]
    def issue(msg):issues.append(str(msg))
    html={p:Links() for p in root.glob('*.html')}
    for p,parsed in html.items():parsed.feed(p.read_text())
    def check_link(p,link):
        nonlocal local_links
        u=urlsplit(link)
        if u.scheme:
            if not link.startswith('https://github.com/fdelillo/crm/blob/'+sha+'/'):
                issue(f'{p.name}: referencia externa fuera de la revisión: {link}')
            else:urls.add(link)
            return
        local_links+=1;target=(p.parent/unquote(u.path)).resolve() if u.path else p.resolve()
        if not target.is_file():issue(f'{p.name}: enlace inexistente: {link}')
        elif u.fragment and target.suffix=='.html' and u.fragment not in html.get(target,Links()).ids:
            issue(f'{p.name}: ancla inexistente: {link}')
    for p,parsed in html.items():
        raw=p.read_text()
        if 'fuentes/' in raw:issue(f'{p.name}: referencia a fuentes eliminadas')
        for link in parsed.hrefs:check_link(p,link)
        for link in parsed.resources:issue(f'{p.name}: recurso externo o no inline: {link}')
        if '<svg ' in raw:
            diagram_pages.append(p);figures+=raw.count('<svg ')
            interactive+=len(re.findall(r'<section[^>]*\bdata-motion-root\b',raw))
            if '<script' in raw and any(word in raw for word in ['Paused at step','Reduced motion ·','Step ${','Ready ·','Complete ·']):
                issue(f'{p.name}: mensaje de estado sin traducir')
    for p in root.glob('*.md'):
        for link in re.findall(r'\]\(([^)]+)\)',p.read_text()):check_link(p,link)
    def scan(value):
        if isinstance(value,dict):
            for x in value.values():scan(x)
        elif isinstance(value,list):
            for x in value:scan(x)
        elif isinstance(value,str) and value.startswith('https://'):check_link(root/'manifest.json',value)
    scan(manifest)
    files={};anchors=0
    for url in sorted(urls):
        u=urlsplit(url);prefix='/fdelillo/crm/blob/'+sha+'/'
        path=unquote(u.path[len(prefix):])
        if path not in files:
            r=subprocess.run(['git','-C',str(args.repo),'cat-file','-e',sha+':'+path],capture_output=True,text=True)
            if r.returncode:issue('git cat-file -e falló: '+path);continue
            data=subprocess.check_output(['git','-C',str(args.repo),'show',sha+':'+path],text=True)
            files[path]=len(data.splitlines())
        if u.fragment:
            m=re.fullmatch(r'L(\d+)(?:-L(\d+))?',u.fragment)
            if not m:issue('Ancla GitHub inválida: '+url)
            elif not 1<=int(m[1])<=int(m[2] or m[1])<=files[path]:issue('Rango fuera del archivo: '+url)
            anchors+=1
    if (root/'fuentes').exists():issue('La carpeta de copias de fuentes todavía existe')
    if len(diagram_pages)!=38 or figures!=59 or interactive!=17:issue('Conteos inesperados')
    original_failures={};localized_passes=0;original_translation_pages=0
    if args.self_check:
        spec=importlib.util.spec_from_file_location('diagram_self_check',args.self_check)
        module=importlib.util.module_from_spec(spec);sys.modules[spec.name]=module;spec.loader.exec_module(module)
        canonical=module.canonical_controller()
        localized=canonical
        for original,translated in TRANSLATIONS.items():
            assert original in localized,original
            localized=localized.replace(original,translated)
        for p in diagram_pages:
            errs=module.verify(p)
            if errs:original_failures[p.name]=errs
        expected_error='script 1 must exactly match the controller in template-motion.html'
        for name,errs in original_failures.items():
            if expected_error in errs:original_translation_pages+=1
            for error in errs:
                if error!=expected_error and not error.startswith('remote reference on <a>: https://github.com/fdelillo/crm/blob/'+sha+'/'):
                    issue(name+': '+error)
        original_reference_error=module.reference_error
        def reference_error(tag,rel,value):
            # Solo se permiten hipervínculos validados previamente; recursos remotos siguen prohibidos.
            if tag=='a' and value in urls:return None
            return original_reference_error(tag,rel,value)
        module.reference_error=reference_error
        # La comparación exacta del controlador sigue activa; solo cambia la referencia de mensajes.
        module.canonical_controller=lambda:localized
        for p in diagram_pages:
            errs=module.verify(p)
            if errs:issue(p.name+': '+str(errs))
            else:localized_passes+=1
    report={'reference_commit':sha,'diagram_pages':len(diagram_pages),'figures':figures,'interactive':interactive,
            'html_files':len(html),'internal_links':local_links,'github_urls':len(urls),'github_files_cat_file_verified':len(files),
            'github_line_anchors_verified':anchors,'self_check_original_passes':len(diagram_pages)-len(original_failures) if args.self_check else None,
            'self_check_original_pages_with_spanish_literal_difference':original_translation_pages,'self_check_original_pages_with_github_link_difference':len(original_failures),'self_check_user_requirements_adapter_passes':localized_passes,
            'issues':issues}
    print(json.dumps(report,ensure_ascii=False,indent=2))
    return bool(issues)

if __name__=='__main__':sys.exit(main())

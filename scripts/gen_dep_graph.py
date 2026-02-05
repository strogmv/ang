import os
import re
import json

def scan_go_deps(root_dir):
    graph = {}
    # Matches: import "pkg" OR import (\n "pkg" \n)
    import_block_re = re.compile(r'import\s*\((.*?)\)', re.DOTALL)
    single_import_re = re.compile(r'import\s+[\w_.]*\s*"(.*?)"')
    inside_block_re = re.compile(r'"(.*?)"')

    for dirpath, _, filenames in os.walk(root_dir):
        if "node_modules" in dirpath or ".git" in dirpath:
            continue
            
        for f in filenames:
            if not f.endswith('.go'): continue
            
            path = os.path.join(dirpath, f)
            # Use file path as key, or package path? Let's use file path for precision
            rel_path = os.path.relpath(path, root_dir)
            
            deps = set()

            try:
                with open(path, 'r', encoding='utf-8') as file:
                    content = file.read()
                    
                    # Single line imports
                    for m in single_import_re.findall(content):
                        deps.add(m)
                    
                    # Block imports
                    for block in import_block_re.findall(content):
                        for m in inside_block_re.findall(block):
                            deps.add(m)
            except Exception as e:
                print(f"Error reading {path}: {e}")
                continue

            graph[rel_path] = list(deps)

    return graph

if __name__ == "__main__":
    g = scan_go_deps(".")
    with open("dependency-graph.json", "w") as f:
        json.dump(g, f, indent=2)
    print(f"Generated dependency-graph.json with {len(g)} files")

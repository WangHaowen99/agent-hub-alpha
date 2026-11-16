from pw import *
with sync_playwright() as p:
    ctx, page = launch(p)
    f = open_workbench(page)
    page.wait_for_timeout(7000)
    out = f.evaluate("""() => ({selected: document.querySelector(".wb-item.on b")?.textContent ?? "", banner: document.querySelector(".wb-err")?.textContent ?? "", chip: document.querySelector(".cg-head .chip:not(.soft)")?.textContent ?? "", items: [...document.querySelectorAll(".wb-item")].map(e => e.innerText.split("\\n").slice(0,2).join(" "))})""")
    shot(page, "z-after-go-restart")
    print(json.dumps(out, ensure_ascii=False))
    ctx.close()

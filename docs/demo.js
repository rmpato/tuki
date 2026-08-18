/* A small, honest replica of tuki's interface.
 *
 * Same grouping, same keys, same behaviour: completed tasks stay in place and
 * get crossed out, the cursor follows the task you were on, and finishing
 * something gets you a quiet reaction and nothing more. */

(function () {
  "use strict";

  var GROUPS = ["work", "home", "hobby", "misc"];

  var CHEERS = [
    "nice", "done!", "tuki approves", "one less thing",
    "good one", "tidy", "less to carry", "gone",
  ];

  var PLACEHOLDERS = [
    "water the plants", "book the dentist", "reply to sam",
    "buy stamps", "learn one more chord", "back up the laptop",
  ];

  var nextId = 100;

  function seed() {
    return [
      { id: 1, text: "write the docs",      tag: "work",  done: false, due: "fri" },
      { id: 2, text: "reply to sam",        tag: "work",  done: false, due: "3d ago", late: true },
      { id: 3, text: "ship the release notes", tag: "work", done: true },
      { id: 4, text: "buy oat milk",        tag: "home",  done: false, due: "tomorrow" },
      { id: 5, text: "water the plants",    tag: "home",  done: true },
      { id: 6, text: "learn one more chord", tag: "hobby", done: false },
    ];
  }

  var body    = document.getElementById("demo-body");
  var foot    = document.getElementById("demo-foot");
  var faceEl  = document.getElementById("demo-face");
  var cheerEl = document.getElementById("demo-cheer");
  var barEl   = document.getElementById("demo-bar");
  var countEl = document.getElementById("demo-count");
  var demo    = document.getElementById("demo");

  if (!demo || !body) return;

  var tasks    = seed();
  var selected = 1;      // task id under the cursor
  var flashId  = null;
  var adding   = false;
  var draft    = "";
  var cheerTimer, flashTimer, faceTimer;

  /* ------------------------------------------------------------ model -- */

  function ordered() {
    var out = [];
    GROUPS.forEach(function (tag) {
      tasks.forEach(function (t) { if (t.tag === tag) out.push(t); });
    });
    // Any tag we didn't anticipate still shows up.
    tasks.forEach(function (t) {
      if (GROUPS.indexOf(t.tag) === -1) out.push(t);
    });
    return out;
  }

  function indexOfSelected() {
    var list = ordered();
    for (var i = 0; i < list.length; i++) {
      if (list[i].id === selected) return i;
    }
    return 0;
  }

  function move(delta) {
    var list = ordered();
    if (!list.length) return;
    var i = Math.min(Math.max(indexOfSelected() + delta, 0), list.length - 1);
    selected = list[i].id;
  }

  function current() {
    var list = ordered();
    for (var i = 0; i < list.length; i++) {
      if (list[i].id === selected) return list[i];
    }
    return null;
  }

  function toggle() {
    var t = current();
    if (!t) return;
    t.done = !t.done;

    if (t.done) {
      flash(t.id);
      cheer(CHEERS[Math.floor(Math.random() * CHEERS.length)]);
      setFace("(^.^)");
    }
    render();
  }

  function remove() {
    var t = current();
    if (!t) return;

    var row = body.querySelector('[data-id="' + t.id + '"]');
    var list = ordered();
    var i = indexOfSelected();

    tasks = tasks.filter(function (x) { return x.id !== t.id; });
    var rest = ordered();
    selected = rest.length ? (rest[Math.min(i, rest.length - 1)] || rest[0]).id : null;

    // Let the row animate out before the list reflows under it.
    if (row) {
      row.classList.add("leaving");
      setTimeout(render, 180);
    } else {
      render();
    }
  }

  function add(text) {
    text = (text || "").trim();
    if (!text) return;

    // "#tag" works here just like it does in the real thing.
    var tag = "misc";
    text = text.replace(/(^|\s)#([^\s]+)/, function (_, pre, found) {
      tag = found.toLowerCase();
      return pre;
    }).trim();
    if (!text) return;

    var t = { id: nextId++, text: text, tag: tag, done: false };
    tasks.push(t);
    selected = t.id;
    render();
  }

  /* --------------------------------------------------------- feedback -- */

  function cheer(word) {
    clearTimeout(cheerTimer);
    cheerEl.textContent = word;
    cheerEl.classList.add("show");
    cheerTimer = setTimeout(function () {
      cheerEl.classList.remove("show");
    }, 1400);
  }

  function flash(id) {
    clearTimeout(flashTimer);
    flashId = id;
    flashTimer = setTimeout(function () {
      flashId = null;
      render();
    }, 850);
  }

  function setFace(face) {
    clearTimeout(faceTimer);
    faceEl.textContent = face;
    faceTimer = setTimeout(restFace, 1400);
  }

  function restFace() {
    var total = tasks.length;
    var done  = tasks.filter(function (t) { return t.done; }).length;
    var late  = tasks.filter(function (t) { return t.late && !t.done; }).length;

    if (!total)            faceEl.textContent = "(u.u)";
    else if (done === total) faceEl.textContent = "(^o^)";
    else if (adding)       faceEl.textContent = "(o.O)";
    else if (late >= 6)    faceEl.textContent = "(-.-)";
    else                   faceEl.textContent = "(o.o)";
  }

  /* ------------------------------------------------------------- view -- */

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function render() {
    body.textContent = "";

    var present = GROUPS.filter(function (tag) {
      return tasks.some(function (t) { return t.tag === tag; });
    });
    tasks.forEach(function (t) {
      if (present.indexOf(t.tag) === -1) present.push(t.tag);
    });

    present.forEach(function (tag) {
      var group = tasks.filter(function (t) { return t.tag === tag; });
      var done  = group.filter(function (t) { return t.done; }).length;

      var head = el("div", "group " + tag);
      head.appendChild(el("span", null, tag.toUpperCase()));
      head.appendChild(el("span", "rule"));
      head.appendChild(el("span", "n", done + "/" + group.length));
      body.appendChild(head);

      group.forEach(function (t) {
        var row = el("div", "task");
        row.dataset.id = t.id;
        if (t.id === selected) row.classList.add("sel");
        if (t.done) row.classList.add("done");
        if (t.late) row.classList.add("late");
        if (t.id === flashId) row.classList.add("flash");

        row.appendChild(el("span", "cursor", t.id === selected ? "▸" : ""));
        row.appendChild(el("span", "mark", t.done ? "✓" : "○"));
        row.appendChild(el("span", "text", t.text));
        row.appendChild(el("span", "due", t.due || ""));

        row.addEventListener("click", function () {
          selected = t.id;
          demo.focus();
          render();
        });
        body.appendChild(row);
      });
    });

    if (!tasks.length) {
      var empty = el("div", "group misc");
      empty.appendChild(el("span", null, "nothing left. press a to add something."));
      body.appendChild(empty);
    }

    renderFoot();
    renderProgress();
    if (!flashId) restFace();
  }

  function renderFoot() {
    foot.textContent = "";

    if (adding) {
      foot.appendChild(el("span", null, "+ "));
      var field = el("span", "field", draft);
      foot.appendChild(field);
      foot.appendChild(el("span", "caret"));
      foot.appendChild(el("span", null,
        draft ? "" : "   " + PLACEHOLDERS[nextId % PLACEHOLDERS.length] + "  #home"));
      return;
    }

    foot.appendChild(el("span", null,
      "↑/k up · ↓/j down · space done · a add · d delete · q quit"));
  }

  function renderProgress() {
    var total = tasks.length;
    var done  = tasks.filter(function (t) { return t.done; }).length;
    var pct   = total ? (done / total) * 100 : 0;

    barEl.style.width = pct + "%";
    countEl.textContent = total ? done + "/" + total + " done" : "nothing to do";
  }

  /* ------------------------------------------------------------- keys -- */

  function handle(key) {
    if (adding) {
      if (key === "Enter")       { add(draft); draft = ""; renderFoot(); return true; }
      if (key === "Escape")      { adding = false; draft = ""; render(); return true; }
      if (key === "Backspace")   { draft = draft.slice(0, -1); renderFoot(); return true; }
      if (key.length === 1)      { draft += key; renderFoot(); return true; }
      return false;
    }

    switch (key) {
      case "ArrowDown": case "j": move(1);  render(); return true;
      case "ArrowUp":   case "k": move(-1); render(); return true;
      case " ": case "Enter": case "x": toggle(); return true;
      case "d": remove(); return true;
      case "a": adding = true; draft = ""; render(); return true;
      case "g": { var l = ordered(); if (l.length) selected = l[0].id; render(); return true; }
      case "G": { var m = ordered(); if (m.length) selected = m[m.length - 1].id; render(); return true; }
      case "r": tasks = seed(); selected = 1; render(); return true;
      default: return false;
    }
  }

  demo.addEventListener("keydown", function (e) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (handle(e.key)) e.preventDefault();
  });

  demo.addEventListener("focus", function () { demo.classList.add("active"); });
  demo.addEventListener("blur", function () {
    demo.classList.remove("active");
    if (adding) { adding = false; draft = ""; render(); }
  });
  demo.addEventListener("click", function () { demo.focus(); });

  // Touch controls, for anyone without a keyboard to hand.
  Array.prototype.forEach.call(
    document.querySelectorAll(".touch-controls button"),
    function (btn) {
      btn.addEventListener("click", function (e) {
        e.stopPropagation();
        var map = { up: "ArrowUp", down: "ArrowDown", space: " ", a: "a", d: "d" };
        handle(map[btn.dataset.key]);
      });
    }
  );

  render();

  /* ------------------------------------------------------ page extras -- */

  // The section bar: appears past the hero, and marks where you are.
  (function topbar() {
    var bar = document.getElementById("topbar");
    var hero = document.querySelector(".hero");
    if (!bar || !hero) return;

    var links = {};
    Array.prototype.forEach.call(bar.querySelectorAll("[href^='#']"), function (a) {
      links[a.getAttribute("href").slice(1)] = a;
    });

    // Show the bar once the hero is out of the way.
    if ("IntersectionObserver" in window) {
      new IntersectionObserver(function (entries) {
        bar.classList.toggle("show", !entries[0].isIntersecting);
      }, { rootMargin: "-60px 0px 0px 0px" }).observe(hero);
    } else {
      bar.classList.add("show");
    }

    var sections = ["look", "try", "install", "using", "why", "mori"]
      .map(function (id) { return document.getElementById(id); })
      .filter(Boolean);

    if (!("IntersectionObserver" in window) || !sections.length) return;

    // Whichever section crosses the middle band of the viewport is "current".
    var visible = {};
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) { visible[e.target.id] = e.isIntersecting; });

      var current = null;
      sections.forEach(function (s) {
        if (visible[s.id] && !current) current = s.id;
      });

      Object.keys(links).forEach(function (id) {
        links[id].removeAttribute("aria-current");
      });
      if (current && links[current]) {
        links[current].setAttribute("aria-current", "true");
      }
    }, { rootMargin: "-45% 0px -45% 0px" });

    sections.forEach(function (s) { io.observe(s); });
  })();

  // Copy button on the install command.
  Array.prototype.forEach.call(document.querySelectorAll(".copy"), function (btn) {
    btn.addEventListener("click", function () {
      var target = document.querySelector(btn.dataset.copy);
      if (!target) return;
      var text = target.textContent;

      var done = function () {
        var was = btn.textContent;
        btn.textContent = "copied";
        btn.classList.add("done");
        setTimeout(function () {
          btn.textContent = was;
          btn.classList.remove("done");
        }, 1600);
      };

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, fallback);
      } else {
        fallback();
      }

      function fallback() {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); done(); } catch (err) { /* nothing to do */ }
        document.body.removeChild(ta);
      }
    });
  });

  // Install tabs.
  Array.prototype.forEach.call(document.querySelectorAll("[data-tabs]"), function (group) {
    var tabs   = group.querySelectorAll("[data-tab]");
    var panels = group.querySelectorAll("[data-panel]");

    Array.prototype.forEach.call(tabs, function (tab) {
      tab.addEventListener("click", function () {
        Array.prototype.forEach.call(tabs, function (t) {
          t.setAttribute("aria-selected", String(t === tab));
        });
        Array.prototype.forEach.call(panels, function (p) {
          p.hidden = p.dataset.panel !== tab.dataset.tab;
        });
      });
    });
  });
})();

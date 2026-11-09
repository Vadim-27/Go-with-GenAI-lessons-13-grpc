**#### LibraTrack — Homework: Lesson 10 / Full-Stack Go with HTMX and go:embed**

**\*\*Time Budget:\*\* Up to 2 hours.**

**\*\*Objective:\*\* Transform the secured \*\*LibraTrack\*\* REST API into a fully operational web application using HTML templates, introduce dynamic HTMX interactivity without frontend JavaScript frameworks, and package the entire application into a single portable binary using `go:embed`, verifying its independence in an isolated environment.**



**---**



**##### PART 1: Manual Tasks (Architecture, Security \& Isolation Verification) \[\~35 mins]**

**\*Tasks requiring critical human design choices, autoescaping security verification, and binary deployment checks.\***



&#x20;**\*\*Task 1.1: Web Layer Architecture \& Autoescaping Audit (Human-only):\*\***

**1. In `design\\\_v10.txt` (or update `README.md`), detail the architectural move to HttpOnly Cookies for JWT storage. Explain why storing JWTs in browser `localStorage` introduces XSS risks for server-rendered apps.**

**2. Perform a manual audit of all template files: confirm `html/template` is used exclusively over `text/template`.**

**3. Verify that `template.HTML` is not used inappropriately on untrusted user or database inputs. Document your audit findings.**

**\* \*\*Task 1.2: Single Binary Isolation Verification Test:\*\***

**1. Compile the application into a statically linked binary: `CGO\\\_ENABLED=0 go build -o libratrack ./cmd/api`.**

**2. Create a clean temporary folder (e.g., `/tmp/verify\\\_libratrack`) and copy \*\*only\*\* the `libratrack` executable into it.**

**3. Execute the binary from this directory (where no `./web` or `./templates` folders exist) and verify via browser that `/catalog` and static CSS/JS load seamlessly.**



**---**



**##### PART 2: Prompt Engineering (Templates, Partials \& HTMX Wiring) \[\~40 mins]**

**\*Crafting precise, detailed prompts to generate UI layouts, partials, and HTMX attributes.\***



**\* \*\*Task 2.1: Prompting Layout and Catalog Page (`layout.html` + `catalog.html` + `book\\\_row.html`):\*\***

**\* Write a prompt to generate the layout `web/templates/layout.html` using `{{block "content" .}}` and the partial `web/templates/partials/book\\\_row.html`.**

**\* \*Requirements:\* The catalog page must display a table of books with availability indicators and a search input form.**

**\* \*Deliverable:\* Save prompt to `prompts.txt` and place generated templates in `web/templates/`.**

**\* \*\*Task 2.2: Prompting HTMX Interactivity (Live Search \& Item Deletion):\*\***

**\* Construct a prompt to generate HTMX attributes and corresponding handlers in `internal/handler/web/`:**

&#x20;   \*\*1. \\\*\\\*Live Search:\\\*\\\* Input element with `hx-get="/web/books"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#book-list"`.\*\*

    \*\*2. \\\*\\\*Delete Row (Librarian):\\\*\\\* Delete button in `book\\\_row.html` with `hx-delete="/web/books/{{.ID}}"`, `hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this book?"`.\*\*


**\* \*Deliverable:\* The delete handler must respond with an empty `200 OK` status to remove the targeted DOM element. Save prompt in `prompts.txt`.**



**---**



**##### PART 3: Agentic AI Use (go:embed Integration \& End-to-End Testing) \[\~45 mins]**

**\*Leveraging an AI coding agent to integrate virtual filesystems and build HTML integration tests.\***



**\* \*\*Task 3.1: Implementing `go:embed` for Templates and Static Assets:\*\***

**\* Instruct your AI agent to apply the `//go:embed` directive in `web/templates.go` and `web/static.go`:**

&#x20;   \*\*\\\* Utilize `embed.FS` and `template.ParseFS` instead of disk-based `ParseGlob`.\*\*

    \*\*\\\* Use `fs.Sub(staticFS, "static")` to strip path prefixes when serving static assets.\*\*


**\* \*Execution:\* Wire the embedded filesystems in `cmd/api/main.go`.**

**\* \*\*Task 3.2: AI-Generated HTML/HTMX Handler Tests:\*\***

**\* Direct your AI agent to write table-driven integration tests in `internal/handler/web/catalog\\\_test.go` using `httptest`.**

**\* \*Test Cases:\***

&#x20;   \*\*1. `GET /catalog` returns `HTTP 200` with HTML containing layout markup and book tables.\*\*

    \*\*2. HTMX request `GET /web/books?q=Go` with `HX-Request: true` header returns only rendered `book\\\_row` HTML fragments.\*\*

    \*\*3. An unauthorized or `reader` role attempt on `hx-delete` endpoints receives `HTTP 403 Forbidden`.\*\*






**---**



**##### Evaluation Criteria \& Deliverables (Self-Check)**

**1. \*\*UI Interactivity:\*\* Browsable catalog with live search and row deletion functioning without full page reloads.**

**2. \*\*Single Binary Verification:\*\* Binary passes the isolation test without external disk dependencies.**

**3. \*\*Security Standards:\*\* JWT handled via HttpOnly cookies; contextual autoescaping remains uncompromised.**

**4. \*\*Test Coverage:\*\* Integration tests validate HTML fragment rendering and RBAC enforcement on HTMX endpoints.**

**5. \*\*Prompt Log:\*\* `prompts.txt` logs all prompt definitions for Tasks 2.1 and 2.2.**


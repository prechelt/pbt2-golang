# PbT (People by Temperament) prototype

Developer prototype of the Plat_Forms 2007 PbT specification, implemented with Python, Django, sqlite, and server-rendered HTML/CSS.

## Functionality sketch

Implemented (core use-case flow):
- User registration with mandatory fields and optional profile attributes
- Login/logout with 1-hour session timeout
- TTT (Trivial Temperament Test) with 40 questions, optional skipped answers, MBTI-style evaluation, and tie handling as specified
- Member search with key filters (not-yet-contact, same country, max distance, motto text, TTT type)
- Member list rendering with required attributes and RCD sending
- Status page for self and other members
  - self: profile editing, in-contact/sent/received lists, accept/reject RCD
  - others: contact details hidden unless relationship is `in_contact`
- 2D member distribution plot as PNG (server-side) with relationship-status colors

Not included:
- Contest/deployment procedures from specification Section 5 (intentionally ignored)
- Legacy-browser requirements from Section 4.2 (replaced by standard HTML/CSS compatible with major modern browsers)
- Full SOAP/WSDL service surface from Section 3 (this prototype focuses on the interactive web portal)

## Architecture sketch

- `pbt/`: Django project settings and root URL config
- `portal/`: main app
  - `models.py`: `MemberProfile`, `TttResult`, `ContactRequest`
  - `ttt.py`: TTT questions and evaluator
  - `views.py`: registration/authentication, TTT, search/member list, status pages, RCD flow, plot image generation
  - `tests.py`: targeted behavior tests
- `templates/portal/`: server-rendered HTML templates
- `static/style.css`: plain CSS styling
- sqlite DB (`db.sqlite3`) for persistence

## Start the program

Prerequisites:
- Python 3.12+
- Poetry

Install and run:

```bash
poetry install
poetry run python manage.py migrate
poetry run python manage.py runserver
```

Open: <http://127.0.0.1:8000/>

Run tests:

```bash
poetry run python manage.py test
```

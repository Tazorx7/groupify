Groupify — Product Requirements Document

Tagline: Pick your topic. Secure your spot.
Version: 1.1
Date: 10 November 2026
Status: MVP specification



1. Product overview

Groupify is a web application that allows students to register for one topic and be assigned to a topic-based group. A course representative (admin) configures the available topics and their capacities, monitors registrations, and manages the registration period.

The core promise is simple: every student chooses exactly one topic, their place is reserved safely, and the system never allows a topic to exceed its configured capacity.

2. Problem statement

Topic selection is often handled through spreadsheets, messaging apps, or informal sign-up forms. These approaches can lead to duplicate registrations, overfilled groups, unclear availability, accidental edits, and exposure of students’ personal information.

Groupify provides one controlled registration process with reliable capacity enforcement and a private admin view.

3. Goals and success criteria

Goals

 ● Let a student register for exactly one topic.
 ● Prevent duplicate registrations.
 ● Prevent topic capacity from ever being exceeded, including during simultaneous sign-ups.
 ● Make a student’s selection final after successful registration.
 ● Keep student identities and membership lists private from other students.
 ● Give the course representative tools to configure topics and manage registrations.
 ● Allow registration to be opened and closed centrally.

Success criteria

 ● No topic ever has more registrations than its configured capacity.
 ● A student cannot create more than one registration.
 ● A student cannot change or delete their own confirmed selection.
 ● Public student-facing views never disclose other students’ names or index numbers.
 ● Admin actions and registration events are auditable.

4. Users and roles

Student

Can:

 ● View active topics.
 ● See each topic’s description, capacity, registered count, remaining spaces, and availability.
 ● Submit their full name and index number and choose one topic.
 ● See confirmation of their own successful registration.

Cannot:

 ● View other students’ names, index numbers, or registration records.
 ● Register for more than one topic.
 ● Change or cancel a confirmed selection.
 ● Register when registration is closed or a topic is full/inactive.

Administrator / course representative

Can:

 ● Sign in to a protected admin dashboard.
 ● Create, edit, activate, and deactivate topics.
 ● Set topic capacities.
 ● Open and close registration.
 ● View all registrations and topic membership lists.
 ● Search, sort, and filter registrations.
 ● Export registration data as CSV.
 ● Review audit events.

Admin access must be protected independently from the student registration flow.

5. Student experience

 1. Student opens the Groupify registration page.
 2. The page displays whether registration is open.
 3. Student reviews the available topics and their remaining spaces.
 4. Student enters their full name and index number.
 5. Student selects exactly one topic.
 6. Student reviews the selection and submits.
 7. The server validates the request and reserves a place atomically.
 8. If successful, the student sees a confirmation containing their submitted details and selected topic.

If the topic becomes full before submission, the student receives a clear message and can choose another available topic. A failed submission must not create a partial registration.

Final selection rule: Once a registration succeeds, the student cannot edit, switch, or cancel it through the student interface. Any exceptional correction must be handled by an authorised administrator and recorded in the audit log.

6. Topic availability and privacy

For each topic, the student-facing page may display:

 ● Topic name.
 ● Topic description.
 ● Capacity.
 ● Number of registered students.
 ● Number of remaining spaces.
 ● Availability status (Available / Full / Unavailable).

It must never display:

 ● Names of students in the topic.● Index numbers of students in the topic.
 ● A public membership list.
 ● Searchable or guessable registration records.

The API must enforce this privacy rule. Hiding member names in the frontend is not sufficient if the backend still exposes them through public endpoints.

7. Admin dashboard

The dashboard should provide:

Overview

 ● Total registrations.
 ● Number of active topics.
 ● Total available spaces.
 ● Full topics.
 ● Registration status (Open / Closed).

Topic management

 ● Create a topic with a name, description, and capacity.
 ● Edit topic details and capacity.
 ● Activate or deactivate a topic.
 ● View current occupancy and remaining spaces.

Capacity changes must be validated. An administrator must not reduce a topic’s capacity below its current number of registrations unless a deliberate, separately authorised reallocation workflow is introduced.

Registration management

 ● View all student registrations.
 ● Search by name or index number.
 ● Filter by topic.
 ● Sort by registration time or student details.
 ● Export the filtered or complete registration list as CSV.

Registration controls

 ● Open registration.
 ● Close registration.
 ● View the current registration state.

Closing registration must be enforced by the backend, not merely by hiding the form in the frontend.

Audit log

Record important actions, including:

 ● Registration created.
 ● Registration period opened or closed.
 ● Topic created, edited, activated, or deactivated.
 ● Topic capacity changed.
 ● Admin correction or removal of a registration, if such an operation is supported.

Each audit event should record the actor, action, affected record, timestamp, and relevant before/after values where appropriate.

8. Functional requirements

|ID   |Requirement                                                                      |
|——|———————————————————————————|
|FR-01|Students can view active topics and public availability information.             |
|FR-02|A student must provide a full name and index number to register.                 |
|FR-03|A student can select exactly one topic.                                          |
|FR-04|A student can have only one registration.                                        |
|FR-05|A successful student selection is final from the student interface.              |
|FR-06|The system rejects registration when registration is closed.                     |
|FR-07|The system rejects registration for inactive or full topics.                     |
|FR-08|Topic capacity must never be exceeded.                                           |
|FR-09|Public endpoints must not expose student membership details.                     |
|FR-10|Admins can manage topics and capacities.                                         |
|FR-11|Admins can view, search, filter, and export registrations.                       |
|FR-12|Admins can open and close registration.                                          |
|FR-13|Important admin and registration actions are recorded in an audit log.           |
|FR-14|Duplicate submissions and retry requests must not create duplicate registrations.|

9. Non-functional requirements

 ● Correctness: Capacity and uniqueness rules are enforced by the database and transactional backend logic.
 ● Security: Admin routes require authentication and authorisation. Inputs are validated server-side.
 ● Privacy: Student records are only available to authorised admins and the relevant student confirmation flow.
 ● Reliability: A failed transaction must leave neither a registration nor a consumed capacity slot.
 ● Performance: Topic availability and registration operations should remain responsive under normal class-sized traffic.
 ● Accessibility: Forms must have labels, keyboard support, readable contrast, and clear validation messages.
 ● Maintainability: Keep business logic separate from HTTP handlers and database access.
 ● Observability: Log errors and important system events without exposing sensitive student data in routine logs.

10. Recommended technology stack

Backend

 ● Language: Go.● HTTP: Go net/http or Chi router.
 ● Database driver: pgx.
 ● Logging: Go log/slog.
 ● Validation: Explicit server-side request validation.
 ● API style: REST over HTTPS.

Database

 ● PostgreSQL as the source of truth for registrations, topics, admin accounts, and audit events.

Frontend

 ● React.
 ● TypeScript.
 ● Tailwind CSS.
 ● React Router.

Deployment

 ● Deploy the Go API as a small application service.
 ● Deploy the frontend as a static web application.
 ● Use a managed PostgreSQL service.
 ● Enforce HTTPS.
 ● Store credentials and secrets in environment variables or the hosting provider’s secret manager.
 ● Do not commit production secrets to Git.

Redis, queues, and microservices are not required for the MVP.

11. Data model

A practical initial schema includes the following tables.

topics

 ● id — primary key.
 ● name — topic name.
 ● description — optional description.
 ● capacity — maximum number of registrations.
 ● current_members — maintained occupancy counter.
 ● is_active — whether students can select the topic.
 ● created_at, updated_at.

Constraints:

 ● capacity > 0.
 ● current_members >= 0.
 ● current_members <= capacity.

registrations

 ● id — primary key.
 ● full_name — submitted student name.
 ● index_number — normalised student index number.
 ● topic_id — foreign key to topics.
 ● created_at — registration timestamp.

Constraints:

 ● Unique constraint on index_number.
 ● Foreign key on topic_id.

Treat index numbers as strings, not integers. This preserves leading zeroes and supports formats containing letters. Define and consistently apply a normalisation rule, such as trimming surrounding whitespace and applying case normalisation where appropriate.

admins

 ● id — primary key.
 ● email or username — unique login identifier.
 ● password_hash — securely hashed password, never plaintext.
 ● created_at, updated_at.

app_settings

 ● Key/value or a dedicated settings record for global registration status.
 ● Include an update timestamp and the admin who made the change where practical.

audit_logs

 ● id — primary key.
 ● actor_admin_id — admin responsible, nullable for system events if necessary.
 ● action — event type.
 ● entity_type and entity_id — affected record.
 ● before_data and after_data — structured change details where appropriate.
 ● created_at.

12. Capacity and concurrency correctness

This is one of Groupify’s most important technical requirements.

A simple frontend check such as “remaining spaces > 0” is not sufficient. Two students may see the last available space and submit at nearly the same time. The backend and database must decide who gets it.

Use a database transaction and an atomic conditional update. Conceptually:

BEGIN;

UPDATE topics

SET current_members = current_members + 1

WHERE id = $1

  AND is_active = TRUE

  AND current_members < capacity

RETURNING id;

— If no row is returned, the topic is inactive or full.

— Otherwise, insert the registration in this same transaction.

INSERT INTO registrations (full_name, index_number, topic_id)

VALUES ($2, $3, $1);

COMMIT;

The actual implementation must also check that registration is globally open and handle all failure cases. If the insert fails—for example, because the index number already exists—the transaction must roll back so the capacity counter is not consumed.

Important implementation rules:

 ● Keep the capacity update and registration insert in the same transaction.
 ● Check the affected row / RETURNING result.
 ● Enforce unique index numbers with a database unique constraint.
 ● Roll back on every error.
 ● Do not rely on an in-memory count or frontend availability state.
 ● Add database constraints as a final line of defence.
 ● Use integration tests with concurrent requests to prove the capacity limit cannot be exceeded.

PostgreSQL’s normal READ COMMITTED isolation level is sufficient for this conditional-update pattern when implemented correctly. The database row update serialises competing updates to the same topic row.

Registration status raceThe global registration-open setting must also be checked server-side in a way that has a defined relationship to concurrent admin close operations. Use a transaction and lock/read strategy that ensures a registration cannot slip through after a close operation has taken effect. Document the intended boundary: a registration already committed before closure remains valid; new registrations after closure are rejected.

Idempotency and retries

A client may lose its connection after the server commits but before receiving the response. Support safe retries so the same request cannot create a second registration or consume another slot. The unique index constraint is essential; an idempotency key can provide clearer retry behaviour.

13. API outline

Example REST endpoints:

Public/student endpoints

 ● GET /api/topics — list active topics with public counts and availability only.
 ● GET /api/registration-status — return whether registration is open.
 ● POST /api/registrations — submit a student’s registration.
 ● GET /api/registrations/me — optional private lookup/confirmation flow, only if a safe verification mechanism is implemented.

Do not create a public endpoint that returns all registrations or topic member lists.

Admin endpoints

 ● POST /api/admin/login — authenticate admin.
 ● POST /api/admin/logout — end admin session.
 ● GET /api/admin/overview — dashboard statistics.
 ● GET /api/admin/topics — list topics with management details.
 ● POST /api/admin/topics — create topic.
 ● PATCH /api/admin/topics/{id} — update topic.
 ● POST /api/admin/topics/{id}/activate — activate topic.
 ● POST /api/admin/topics/{id}/deactivate — deactivate topic.
 ● GET /api/admin/registrations — search/filter registrations.
 ● GET /api/admin/registrations/export — export CSV.
 ● GET /api/admin/settings — read registration settings.
 ● PATCH /api/admin/settings — update registration settings.
 ● GET /api/admin/audit-logs — review audit events.

All /api/admin/* endpoints must require an authenticated and authorised admin session.

14. Security and privacy

 ● Hash admin passwords using a modern password-hashing algorithm such as Argon2id or bcrypt with appropriate parameters.
 ● Use secure, HTTP-only, SameSite cookies for browser sessions, with CSRF protection where applicable.
 ● Apply rate limits to login and public registration endpoints.
 ● Validate and normalise all input on the server.
 ● Use parameterised SQL queries.
 ● Protect against SQL injection, cross-site scripting, and cross-site request forgery.
 ● Use HTTPS in production.
 ● Keep secrets out of source control.
 ● Restrict student data to admin-only endpoints.
 ● Avoid logging full index numbers or other personal data unless operationally necessary.
 ● Apply least privilege to database and hosting credentials.
 ● Record sensitive admin actions in the audit log.

Identity verification limitation

A full name and index number do not prove that the person submitting the form owns that index number. If impersonation is a concern, the course representative should preload an approved class roster and verify submitted index numbers against it. A stronger option is a one-time verification code sent through an institution-controlled channel. The appropriate method depends on the available student data and institutional requirements.

15. Validation and error handling

The API should return clear, consistent errors for cases such as:

 ● Registration is closed.
 ● Index number is missing or invalid.
 ● Index number has already registered.
 ● Topic does not exist.
 ● Topic is inactive.
 ● Topic is full.
 ● Request data is invalid.
 ● Admin authentication is required.

Never return internal database errors, stack traces, secrets, or other students’ personal information to the client.

16. Testing strategy

Unit tests

 ● Input validation and index number normalisation.
 ● Registration status rules.
 ● Topic availability rules.
 ● Admin authorisation.
 ● Error mapping.

Integration tests

 ● Successful registration.
 ● Duplicate index number rejection.● Registration rejected when closed.
 ● Registration rejected for inactive topics.
 ● Registration rejected when full.
 ● Capacity counter rolls back if registration insertion fails.
 ● Student-facing endpoints do not expose membership details.

Concurrency tests

Create a topic with a small capacity, then send more simultaneous registration requests than available spaces. Verify:

 ● Successful registrations never exceed capacity.
 ● Only the allowed number of requests succeed.
 ● No failed request consumes a slot.
 ● No index number is registered twice.

End-to-end tests

 ● Student completes a registration.
 ● Student cannot submit a second registration.
 ● Admin creates a topic and changes its capacity.
 ● Admin closes registration and student submission is rejected.
 ● Admin searches registrations and exports CSV.

17. User interface requirements

Student pages

 ● Registration landing page.
 ● Topic selection list/cards.
 ● Registration form.
 ● Submission review and confirmation.
 ● Clear full/closed/error states.

Admin pages

 ● Admin login.
 ● Dashboard overview.
 ● Topic management.
 ● Registration management.
 ● Registration settings.
 ● Audit log.

The interface should be responsive and usable on mobile devices, since students may register using their phones.

18. MVP scope and exclusions

Included in MVP

 ● One administrator role.
 ● Student registration with name and index number.
 ● Exactly one topic per student.
 ● Topic capacity management.
 ● Atomic capacity enforcement.
 ● Registration open/close control.
 ● Private admin registration list.
 ● Search and CSV export.
 ● Basic audit logging.
 ● Responsive student and admin interfaces.

Not required for MVP

 ● Multiple courses or departments.
 ● Student-to-student messaging.
 ● Group chat.
 ● Automated group assignment.
 ● Payment processing.
 ● Native mobile applications.
 ● Complex role hierarchies.
 ● Redis, message brokers, or microservices.

These can be considered later if actual usage requires them.

19. Suggested repository structure

groupify/

├── backend/

│   ├── cmd/

│   │   └── api/

│   │       └── main.go

│   ├── internal/

│   │   ├── auth/

│   │   ├── students/

│   │   ├── topics/

│   │   ├── registrations/

│   │   ├── admin/

│   │   ├── database/

│   │   ├── middleware/

│   │   └── audit/

│   └── migrations/

├── frontend/

│   └── src/

│       ├── components/

│       ├── pages/

│       │   ├── student/

│       │   └── admin/

│       ├── hooks/

│       ├── services/

│       ├── layouts/

│       └── types/

├── docs/

│   ├── api.md

│   ├── database.md

│   └── deployment.md

├── docker-compose.yml

├── .env.example

└── README.md

20. Delivery milestones

 1. Foundation: Repository setup, database schema, migrations, configuration, and health endpoint.
 2. Core registration: Topic listing, registration endpoint, validation, unique index constraint, and transactional capacity enforcement.
 3. Student interface: Topic availability, registration form, confirmation, and clear error states.
 4. Admin foundation: Authentication, topic management, registration open/close controls.
 5. Admin operations: Registration search, filters, CSV export, and audit log.
 6. Hardening: Concurrency tests, security review, accessibility checks, and deployment.

21. Acceptance criteria

The MVP is ready when:

 ● A student can register for one active topic while registration is open.
 ● A student cannot register twice using the same normalised index number.
 ● A student cannot change their confirmed topic through the student interface.
 ● No topic can exceed its configured capacity, even under concurrent submissions.
 ● A failed registration does not consume a capacity slot.
 ● Students cannot access other students’ names, index numbers, or membership records.
 ● An administrator can create and manage topics and capacities.
 ● An administrator can open and close registration.
 ● The backend rejects registrations while registration is closed.
 ● An administrator can search registrations and export them as CSV.● Important administrative changes are recorded in the audit log.



Product principle: Groupify should remain simple for students and dependable behind the scenes. The student experience is a quick topic choice; the backend is responsible for making that choice fair, private, and final.

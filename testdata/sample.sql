-- sqly サンプルデータ
-- 使い方: sqly 起動後にエディタに貼り付けて実行

CREATE TABLE departments (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL
);

CREATE TABLE employees (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    department_id INTEGER NOT NULL REFERENCES departments(id),
    salary INTEGER NOT NULL
);

CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    lead_id INTEGER NOT NULL REFERENCES employees(id),
    budget INTEGER NOT NULL
);

INSERT INTO departments (id, name, location) VALUES
    (1, 'Engineering', 'Tokyo'),
    (2, 'Sales', 'Osaka'),
    (3, 'HR', 'Tokyo'),
    (4, 'Marketing', 'Fukuoka');

INSERT INTO employees (id, name, department_id, salary) VALUES
    (1, 'Tanaka Yuki', 1, 7500000),
    (2, 'Suzuki Hana', 1, 6800000),
    (3, 'Sato Ren', 2, 6000000),
    (4, 'Yamamoto Aoi', 3, 5500000),
    (5, 'Nakamura Sota', 1, 8200000),
    (6, 'Ito Mei', 2, 5800000),
    (7, 'Watanabe Haruto', 4, 6200000),
    (8, 'Kobayashi Sakura', 3, 5200000);

INSERT INTO projects (id, title, lead_id, budget) VALUES
    (1, 'sqly v2.0', 1, 5000000),
    (2, 'Customer Portal', 5, 12000000),
    (3, 'New Hire Onboarding', 4, 2000000),
    (4, 'Brand Refresh', 7, 3500000);

-- クエリ例:
-- SELECT e.name, d.name AS department, e.salary FROM employees e JOIN departments d ON e.department_id = d.id ORDER BY e.salary DESC;
-- SELECT d.name, COUNT(*) AS headcount, AVG(e.salary) AS avg_salary FROM employees e JOIN departments d ON e.department_id = d.id GROUP BY d.name;
-- SELECT p.title, e.name AS lead, p.budget FROM projects p JOIN employees e ON p.lead_id = e.id;

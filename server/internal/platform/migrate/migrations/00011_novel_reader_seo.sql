-- +goose Up
-- +goose StatementBegin
CREATE TABLE novel_reader_seo_config (
    id bigint PRIMARY KEY,
    seo_enabled boolean NOT NULL DEFAULT true,
    indexing_enabled boolean NOT NULL DEFAULT true,
    sitemap_enabled boolean NOT NULL DEFAULT true,
    site_name varchar(100) NOT NULL,
    site_url varchar(2048) NOT NULL,
    default_description varchar(2000) NOT NULL,
    home_title varchar(500) NOT NULL,
    home_description varchar(2000) NOT NULL,
    books_title_template varchar(500) NOT NULL,
    books_description_template varchar(2000) NOT NULL,
    book_title_template varchar(500) NOT NULL,
    book_description_template varchar(2000) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT novel_reader_seo_singleton_check CHECK (id = 1),
    CONSTRAINT novel_reader_seo_text_not_blank CHECK (
        btrim(site_name) <> '' AND btrim(site_url) <> '' AND
        btrim(default_description) <> '' AND btrim(home_title) <> '' AND
        btrim(home_description) <> '' AND btrim(books_title_template) <> '' AND
        btrim(books_description_template) <> '' AND btrim(book_title_template) <> '' AND
        btrim(book_description_template) <> ''
    )
);

INSERT INTO novel_reader_seo_config
    (id, seo_enabled, indexing_enabled, sitemap_enabled, site_name, site_url,
     default_description, home_title, home_description, books_title_template,
     books_description_template, book_title_template, book_description_template)
VALUES
    (1, true, true, true, '月白书城', 'https://ybsc.me',
     '月白书城提供精选小说在线阅读与作品发现。',
     '月白书城 - 精选小说在线阅读',
     '在月白书城发现精选小说，浏览作品分类并开始阅读。',
     '书库 - {siteName}',
     '浏览{siteName}书库，发现不同分类的精选小说。',
     '{bookName} - {authorName} - {siteName}',
     '《{bookName}》由{authorName}创作，{bookDesc}');

INSERT INTO sys_base_menus
    (id, created_at, updated_at, menu_level, parent_id, path, name, hidden, component, sort,
     active_name, keep_alive, default_menu, title, icon, close_tab, transition_type)
VALUES (1005, now(), now(), 1, 1000, 'readerSeo', 'NovelReaderSeo', false,
    'view/novel/readerSeo/index.vue', 5, '', true, false, 'SEO设置', 'promotion', false, '')
ON CONFLICT DO NOTHING;

INSERT INTO sys_authority_menus (sys_base_menu_id, sys_authority_authority_id)
VALUES (1005, 888) ON CONFLICT DO NOTHING;

INSERT INTO sys_apis (id, created_at, updated_at, path, description, api_group, method)
VALUES
    (1021, now(), now(), '/novel/readerSeo/config', '查询读者SEO配置', '读者SEO', 'GET'),
    (1022, now(), now(), '/novel/readerSeo/config', '修改读者SEO配置', '读者SEO', 'PUT')
ON CONFLICT DO NOTHING;

INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4, v5)
VALUES
    ('p','888','/novel/readerSeo/config','GET','','',''),
    ('p','888','/novel/readerSeo/config','PUT','','','')
ON CONFLICT DO NOTHING;

SELECT setval('sys_base_menus_id_seq', GREATEST((SELECT max(id) FROM sys_base_menus), 1005), true);
SELECT setval('sys_apis_id_seq', GREATEST((SELECT max(id) FROM sys_apis), 1022), true);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration'; END $$;
-- +goose StatementEnd

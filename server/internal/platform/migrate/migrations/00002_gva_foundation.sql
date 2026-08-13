-- +goose Up
-- +goose StatementBegin

CREATE TABLE public.casbin_rule (
    id bigint NOT NULL,
    ptype character varying(100),
    v0 character varying(100),
    v1 character varying(100),
    v2 character varying(100),
    v3 character varying(100),
    v4 character varying(100),
    v5 character varying(100)
);

CREATE SEQUENCE public.casbin_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.casbin_rule_id_seq OWNED BY public.casbin_rule.id;

CREATE TABLE public.exa_customers (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    customer_name text,
    customer_phone_data text,
    sys_user_id bigint,
    sys_user_authority_id bigint,
    dept_id bigint,
    created_by bigint
);

COMMENT ON COLUMN public.exa_customers.customer_name IS '客户名';

COMMENT ON COLUMN public.exa_customers.customer_phone_data IS '客户手机号';

COMMENT ON COLUMN public.exa_customers.sys_user_id IS '管理ID';

COMMENT ON COLUMN public.exa_customers.sys_user_authority_id IS '管理角色ID';

COMMENT ON COLUMN public.exa_customers.dept_id IS '归属部门ID(数据权限)';

COMMENT ON COLUMN public.exa_customers.created_by IS '创建人(数据权限)';

CREATE SEQUENCE public.exa_customers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.exa_customers_id_seq OWNED BY public.exa_customers.id;

CREATE TABLE public.gva_announcements_info (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    title text,
    content text,
    user_id bigint,
    attachments jsonb
);

COMMENT ON COLUMN public.gva_announcements_info.title IS '公告标题';

COMMENT ON COLUMN public.gva_announcements_info.content IS '公告内容';

COMMENT ON COLUMN public.gva_announcements_info.user_id IS '发布者';

COMMENT ON COLUMN public.gva_announcements_info.attachments IS '相关附件';

CREATE SEQUENCE public.gva_announcements_info_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.gva_announcements_info_id_seq OWNED BY public.gva_announcements_info.id;

CREATE TABLE public.jwt_blacklists (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    jwt text
);

COMMENT ON COLUMN public.jwt_blacklists.jwt IS 'jwt';

CREATE SEQUENCE public.jwt_blacklists_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.jwt_blacklists_id_seq OWNED BY public.jwt_blacklists.id;

CREATE TABLE public.media_attachment_category (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name character varying(255) DEFAULT NULL::character varying,
    pid bigint DEFAULT 0
);

COMMENT ON COLUMN public.media_attachment_category.name IS '分类名称';

COMMENT ON COLUMN public.media_attachment_category.pid IS '父节点ID';

CREATE SEQUENCE public.media_attachment_category_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.media_attachment_category_id_seq OWNED BY public.media_attachment_category.id;

CREATE TABLE public.media_file_upload_and_downloads (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    class_id bigint DEFAULT 0,
    url text,
    tag text,
    key text,
    size bigint DEFAULT 0,
    mime character varying(255),
    md5 character varying(64),
    user_id bigint
);

COMMENT ON COLUMN public.media_file_upload_and_downloads.name IS '文件名';

COMMENT ON COLUMN public.media_file_upload_and_downloads.class_id IS '分类id';

COMMENT ON COLUMN public.media_file_upload_and_downloads.url IS '文件地址';

COMMENT ON COLUMN public.media_file_upload_and_downloads.tag IS '文件标签';

COMMENT ON COLUMN public.media_file_upload_and_downloads.key IS '编号';

COMMENT ON COLUMN public.media_file_upload_and_downloads.size IS '文件大小(字节)';

COMMENT ON COLUMN public.media_file_upload_and_downloads.mime IS 'MIME类型';

COMMENT ON COLUMN public.media_file_upload_and_downloads.md5 IS '文件MD5';

COMMENT ON COLUMN public.media_file_upload_and_downloads.user_id IS '上传者ID';

CREATE SEQUENCE public.media_file_upload_and_downloads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.media_file_upload_and_downloads_id_seq OWNED BY public.media_file_upload_and_downloads.id;

CREATE TABLE public.media_upload_chunks (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    upload_id bigint,
    chunk_index bigint,
    chunk_hash text,
    size bigint
);

CREATE SEQUENCE public.media_upload_chunks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.media_upload_chunks_id_seq OWNED BY public.media_upload_chunks.id;

CREATE TABLE public.media_uploads (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id bigint,
    file_name text,
    file_hash text,
    file_size bigint,
    chunk_size bigint,
    chunk_total bigint,
    status text DEFAULT 'uploading'::text,
    storage_key text,
    media_id bigint
);

COMMENT ON COLUMN public.media_uploads.user_id IS '上传者';

COMMENT ON COLUMN public.media_uploads.file_name IS '文件名';

COMMENT ON COLUMN public.media_uploads.file_hash IS '整文件MD5';

COMMENT ON COLUMN public.media_uploads.file_size IS '总字节';

COMMENT ON COLUMN public.media_uploads.chunk_size IS '分片字节';

COMMENT ON COLUMN public.media_uploads.chunk_total IS '分片总数';

COMMENT ON COLUMN public.media_uploads.status IS '状态';

COMMENT ON COLUMN public.media_uploads.storage_key IS '最终对象key';

COMMENT ON COLUMN public.media_uploads.media_id IS '媒体库记录ID';

CREATE SEQUENCE public.media_uploads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.media_uploads_id_seq OWNED BY public.media_uploads.id;

CREATE TABLE public.sys_api_tokens (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id bigint,
    authority_id bigint,
    token text,
    status boolean DEFAULT true,
    expires_at timestamp with time zone,
    remark text
);

COMMENT ON COLUMN public.sys_api_tokens.user_id IS '用户ID';

COMMENT ON COLUMN public.sys_api_tokens.authority_id IS '角色ID';

COMMENT ON COLUMN public.sys_api_tokens.token IS 'Token';

COMMENT ON COLUMN public.sys_api_tokens.status IS '状态';

COMMENT ON COLUMN public.sys_api_tokens.expires_at IS '过期时间';

COMMENT ON COLUMN public.sys_api_tokens.remark IS '备注';

CREATE SEQUENCE public.sys_api_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_api_tokens_id_seq OWNED BY public.sys_api_tokens.id;

CREATE TABLE public.sys_apis (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    path text,
    description text,
    api_group text,
    method text DEFAULT 'POST'::text
);

COMMENT ON COLUMN public.sys_apis.path IS 'api路径';

COMMENT ON COLUMN public.sys_apis.description IS 'api中文描述';

COMMENT ON COLUMN public.sys_apis.api_group IS 'api组';

COMMENT ON COLUMN public.sys_apis.method IS '方法';

CREATE SEQUENCE public.sys_apis_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_apis_id_seq OWNED BY public.sys_apis.id;

CREATE TABLE public.sys_authorities (
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    authority_id bigint NOT NULL,
    authority_name text,
    parent_id bigint,
    data_scope bigint DEFAULT 1,
    default_router text DEFAULT 'dashboard'::text
);

COMMENT ON COLUMN public.sys_authorities.authority_id IS '角色ID';

COMMENT ON COLUMN public.sys_authorities.authority_name IS '角色名';

COMMENT ON COLUMN public.sys_authorities.parent_id IS '父角色ID';

COMMENT ON COLUMN public.sys_authorities.data_scope IS '数据范围 1全部 2本部门及子级 3本部门 4仅本人';

COMMENT ON COLUMN public.sys_authorities.default_router IS '默认菜单';

CREATE SEQUENCE public.sys_authorities_authority_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_authorities_authority_id_seq OWNED BY public.sys_authorities.authority_id;

CREATE TABLE public.sys_authority_btns (
    authority_id bigint,
    sys_menu_id bigint,
    sys_base_menu_btn_id bigint
);

COMMENT ON COLUMN public.sys_authority_btns.authority_id IS '角色ID';

COMMENT ON COLUMN public.sys_authority_btns.sys_menu_id IS '菜单ID';

COMMENT ON COLUMN public.sys_authority_btns.sys_base_menu_btn_id IS '菜单按钮ID';

CREATE TABLE public.sys_authority_departments (
    sys_authority_authority_id bigint,
    sys_department_id bigint
);

CREATE TABLE public.sys_authority_menus (
    sys_base_menu_id bigint NOT NULL,
    sys_authority_authority_id bigint NOT NULL
);

COMMENT ON COLUMN public.sys_authority_menus.sys_authority_authority_id IS '角色ID';

CREATE TABLE public.sys_auto_code_histories (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    table_name text,
    package text,
    request text,
    struct_name text,
    abbreviation text,
    business_db text,
    description text,
    templates text,
    injections text,
    flag bigint,
    api_ids text,
    menu_id bigint,
    export_template_id bigint,
    package_id bigint
);

COMMENT ON COLUMN public.sys_auto_code_histories.table_name IS '表名';

COMMENT ON COLUMN public.sys_auto_code_histories.package IS '模块名或插件名';

COMMENT ON COLUMN public.sys_auto_code_histories.request IS '前端传入的结构化信息';

COMMENT ON COLUMN public.sys_auto_code_histories.struct_name IS '结构体名称';

COMMENT ON COLUMN public.sys_auto_code_histories.abbreviation IS '结构体简称';

COMMENT ON COLUMN public.sys_auto_code_histories.business_db IS '业务库';

COMMENT ON COLUMN public.sys_auto_code_histories.description IS '结构体中文名';

COMMENT ON COLUMN public.sys_auto_code_histories.templates IS '模板信息';

COMMENT ON COLUMN public.sys_auto_code_histories.injections IS '注入信息';

COMMENT ON COLUMN public.sys_auto_code_histories.flag IS '[0:创建,1:回滚]';

COMMENT ON COLUMN public.sys_auto_code_histories.api_ids IS '关联API ID';

COMMENT ON COLUMN public.sys_auto_code_histories.menu_id IS '菜单ID';

COMMENT ON COLUMN public.sys_auto_code_histories.export_template_id IS '导出模板ID';

COMMENT ON COLUMN public.sys_auto_code_histories.package_id IS '包ID';

CREATE SEQUENCE public.sys_auto_code_histories_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_auto_code_histories_id_seq OWNED BY public.sys_auto_code_histories.id;

CREATE TABLE public.sys_auto_code_packages (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    "desc" text,
    label text,
    template text,
    package_name text,
    module text
);

COMMENT ON COLUMN public.sys_auto_code_packages."desc" IS '描述';

COMMENT ON COLUMN public.sys_auto_code_packages.label IS '显示名称';

COMMENT ON COLUMN public.sys_auto_code_packages.template IS '模板';

COMMENT ON COLUMN public.sys_auto_code_packages.package_name IS '包名';

CREATE SEQUENCE public.sys_auto_code_packages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_auto_code_packages_id_seq OWNED BY public.sys_auto_code_packages.id;

CREATE TABLE public.sys_base_menu_btns (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    "desc" text,
    sys_base_menu_id bigint
);

COMMENT ON COLUMN public.sys_base_menu_btns.name IS '按钮关键key';

COMMENT ON COLUMN public.sys_base_menu_btns.sys_base_menu_id IS '菜单ID';

CREATE SEQUENCE public.sys_base_menu_btns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_base_menu_btns_id_seq OWNED BY public.sys_base_menu_btns.id;

CREATE TABLE public.sys_base_menu_parameters (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    sys_base_menu_id bigint,
    type text,
    key text,
    value text
);

COMMENT ON COLUMN public.sys_base_menu_parameters.type IS '地址栏携带参数为params还是query';

COMMENT ON COLUMN public.sys_base_menu_parameters.key IS '地址栏携带参数的key';

COMMENT ON COLUMN public.sys_base_menu_parameters.value IS '地址栏携带参数的值';

CREATE SEQUENCE public.sys_base_menu_parameters_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_base_menu_parameters_id_seq OWNED BY public.sys_base_menu_parameters.id;

CREATE TABLE public.sys_base_menus (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    menu_level bigint,
    parent_id bigint,
    path text,
    name text,
    hidden boolean,
    component text,
    sort bigint,
    active_name text,
    keep_alive boolean,
    default_menu boolean,
    title text,
    icon text,
    close_tab boolean,
    transition_type text
);

COMMENT ON COLUMN public.sys_base_menus.parent_id IS '父菜单ID';

COMMENT ON COLUMN public.sys_base_menus.path IS '路由path';

COMMENT ON COLUMN public.sys_base_menus.name IS '路由name';

COMMENT ON COLUMN public.sys_base_menus.hidden IS '是否在列表隐藏';

COMMENT ON COLUMN public.sys_base_menus.component IS '对应前端文件路径';

COMMENT ON COLUMN public.sys_base_menus.sort IS '排序标记';

COMMENT ON COLUMN public.sys_base_menus.active_name IS '高亮菜单';

COMMENT ON COLUMN public.sys_base_menus.keep_alive IS '是否缓存';

COMMENT ON COLUMN public.sys_base_menus.default_menu IS '是否是基础路由（开发中）';

COMMENT ON COLUMN public.sys_base_menus.title IS '菜单名';

COMMENT ON COLUMN public.sys_base_menus.icon IS '菜单图标';

COMMENT ON COLUMN public.sys_base_menus.close_tab IS '自动关闭tab';

COMMENT ON COLUMN public.sys_base_menus.transition_type IS '路由切换动画';

CREATE SEQUENCE public.sys_base_menus_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_base_menus_id_seq OWNED BY public.sys_base_menus.id;

CREATE TABLE public.sys_cli_apis (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    cli_id bigint NOT NULL,
    api_id bigint NOT NULL,
    command_name character varying(128),
    command_desc text,
    params_override text,
    api_brief character varying(255),
    response_override text,
    enabled boolean DEFAULT true NOT NULL,
    sort bigint DEFAULT 0 NOT NULL
);

COMMENT ON COLUMN public.sys_cli_apis.cli_id IS 'CLI ID';

COMMENT ON COLUMN public.sys_cli_apis.api_id IS 'API ID';

COMMENT ON COLUMN public.sys_cli_apis.command_name IS '命令名覆盖';

COMMENT ON COLUMN public.sys_cli_apis.command_desc IS '命令说明覆盖';

COMMENT ON COLUMN public.sys_cli_apis.params_override IS '参数定义覆盖JSON';

COMMENT ON COLUMN public.sys_cli_apis.api_brief IS 'API简介覆盖';

COMMENT ON COLUMN public.sys_cli_apis.response_override IS '返回字段定义覆盖JSON';

COMMENT ON COLUMN public.sys_cli_apis.enabled IS '是否启用';

COMMENT ON COLUMN public.sys_cli_apis.sort IS '排序';

CREATE SEQUENCE public.sys_cli_apis_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_cli_apis_id_seq OWNED BY public.sys_cli_apis.id;

CREATE TABLE public.sys_clis (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name character varying(128) NOT NULL,
    command character varying(128) DEFAULT ''::character varying NOT NULL,
    display_name character varying(128) NOT NULL,
    version character varying(64) DEFAULT 'v1'::character varying NOT NULL,
    description text,
    status character varying(32) DEFAULT 'enabled'::character varying NOT NULL,
    auth_mode character varying(32) DEFAULT 'jwt'::character varying NOT NULL,
    skill_name character varying(128),
    skill_description text,
    scenarios_json text
);

COMMENT ON COLUMN public.sys_clis.name IS 'CLI唯一标识';

COMMENT ON COLUMN public.sys_clis.command IS 'CLI主命令';

COMMENT ON COLUMN public.sys_clis.display_name IS 'CLI展示名称';

COMMENT ON COLUMN public.sys_clis.version IS 'CLI版本';

COMMENT ON COLUMN public.sys_clis.description IS 'CLI描述';

COMMENT ON COLUMN public.sys_clis.status IS 'CLI状态';

COMMENT ON COLUMN public.sys_clis.auth_mode IS '认证方式';

COMMENT ON COLUMN public.sys_clis.skill_name IS 'AI Skill名称';

COMMENT ON COLUMN public.sys_clis.skill_description IS 'AI Skill描述';

COMMENT ON COLUMN public.sys_clis.scenarios_json IS '调用场景链路JSON';

CREATE SEQUENCE public.sys_clis_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_clis_id_seq OWNED BY public.sys_clis.id;

CREATE TABLE public.sys_data_access_logs (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    event_type text,
    target_table text,
    operation text,
    user_id bigint,
    authority_id bigint,
    scope bigint,
    request_id text,
    method text,
    path text,
    detail text
);

COMMENT ON COLUMN public.sys_data_access_logs.event_type IS '事件类型 no_identity/blocked_write';

COMMENT ON COLUMN public.sys_data_access_logs.target_table IS '受控业务表名';

COMMENT ON COLUMN public.sys_data_access_logs.operation IS '操作 query/create/update/delete';

COMMENT ON COLUMN public.sys_data_access_logs.user_id IS '事发用户ID(无身份事件为0)';

COMMENT ON COLUMN public.sys_data_access_logs.authority_id IS '事发角色ID';

COMMENT ON COLUMN public.sys_data_access_logs.scope IS '事发时数据权限档位';

COMMENT ON COLUMN public.sys_data_access_logs.request_id IS '请求ID';

COMMENT ON COLUMN public.sys_data_access_logs.method IS 'HTTP方法';

COMMENT ON COLUMN public.sys_data_access_logs.path IS '请求路径';

COMMENT ON COLUMN public.sys_data_access_logs.detail IS '详情';

CREATE SEQUENCE public.sys_data_access_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_data_access_logs_id_seq OWNED BY public.sys_data_access_logs.id;

CREATE TABLE public.sys_departments (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    parent_id bigint DEFAULT 0,
    ancestors text,
    sort bigint DEFAULT 0,
    leader_id bigint,
    status boolean DEFAULT true
);

COMMENT ON COLUMN public.sys_departments.name IS '部门名称';

COMMENT ON COLUMN public.sys_departments.parent_id IS '父部门ID';

COMMENT ON COLUMN public.sys_departments.ancestors IS '祖级链,逗号分隔如 0,1,5';

COMMENT ON COLUMN public.sys_departments.sort IS '排序';

COMMENT ON COLUMN public.sys_departments.leader_id IS '负责人用户ID';

COMMENT ON COLUMN public.sys_departments.status IS '是否启用';

CREATE SEQUENCE public.sys_departments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_departments_id_seq OWNED BY public.sys_departments.id;

CREATE TABLE public.sys_dictionaries (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    type text,
    status boolean,
    "desc" text,
    parent_id bigint
);

COMMENT ON COLUMN public.sys_dictionaries.name IS '字典名（中）';

COMMENT ON COLUMN public.sys_dictionaries.type IS '字典名（英）';

COMMENT ON COLUMN public.sys_dictionaries.status IS '状态';

COMMENT ON COLUMN public.sys_dictionaries."desc" IS '描述';

COMMENT ON COLUMN public.sys_dictionaries.parent_id IS '父级字典ID';

CREATE SEQUENCE public.sys_dictionaries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_dictionaries_id_seq OWNED BY public.sys_dictionaries.id;

CREATE TABLE public.sys_dictionary_details (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    label text,
    value text,
    extend text,
    status boolean,
    sort bigint,
    sys_dictionary_id bigint,
    parent_id bigint,
    level bigint,
    path text
);

COMMENT ON COLUMN public.sys_dictionary_details.label IS '展示值';

COMMENT ON COLUMN public.sys_dictionary_details.value IS '字典值';

COMMENT ON COLUMN public.sys_dictionary_details.extend IS '扩展值';

COMMENT ON COLUMN public.sys_dictionary_details.status IS '启用状态';

COMMENT ON COLUMN public.sys_dictionary_details.sort IS '排序标记';

COMMENT ON COLUMN public.sys_dictionary_details.sys_dictionary_id IS '关联标记';

COMMENT ON COLUMN public.sys_dictionary_details.parent_id IS '父级字典详情ID';

COMMENT ON COLUMN public.sys_dictionary_details.level IS '层级深度';

COMMENT ON COLUMN public.sys_dictionary_details.path IS '层级路径';

CREATE SEQUENCE public.sys_dictionary_details_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_dictionary_details_id_seq OWNED BY public.sys_dictionary_details.id;

CREATE TABLE public.sys_error (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    form text,
    info text,
    level text,
    request_id character varying(64),
    trace_id character varying(64),
    solution text,
    status character varying(20) DEFAULT '未处理'::character varying
);

COMMENT ON COLUMN public.sys_error.form IS '错误来源';

COMMENT ON COLUMN public.sys_error.info IS '错误内容';

COMMENT ON COLUMN public.sys_error.level IS '日志等级';

COMMENT ON COLUMN public.sys_error.request_id IS '请求ID';

COMMENT ON COLUMN public.sys_error.trace_id IS '链路ID';

COMMENT ON COLUMN public.sys_error.solution IS '解决方案';

COMMENT ON COLUMN public.sys_error.status IS '处理状态';

CREATE SEQUENCE public.sys_error_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_error_id_seq OWNED BY public.sys_error.id;

CREATE TABLE public.sys_export_template_condition (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    template_id text,
    "from" text,
    "column" text,
    operator text
);

COMMENT ON COLUMN public.sys_export_template_condition.template_id IS '模板标识';

COMMENT ON COLUMN public.sys_export_template_condition."from" IS '条件取的key';

COMMENT ON COLUMN public.sys_export_template_condition."column" IS '作为查询条件的字段';

COMMENT ON COLUMN public.sys_export_template_condition.operator IS '操作符';

CREATE SEQUENCE public.sys_export_template_condition_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_export_template_condition_id_seq OWNED BY public.sys_export_template_condition.id;

CREATE TABLE public.sys_export_template_join (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    template_id text,
    joins text,
    "table" text,
    "on" text
);

COMMENT ON COLUMN public.sys_export_template_join.template_id IS '模板标识';

COMMENT ON COLUMN public.sys_export_template_join.joins IS '关联';

COMMENT ON COLUMN public.sys_export_template_join."table" IS '关联表';

COMMENT ON COLUMN public.sys_export_template_join."on" IS '关联条件';

CREATE SEQUENCE public.sys_export_template_join_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_export_template_join_id_seq OWNED BY public.sys_export_template_join.id;

CREATE TABLE public.sys_export_templates (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    db_name text,
    name text,
    table_name text,
    template_id text,
    template_info text,
    sql text,
    import_sql text,
    "limit" bigint,
    "order" text
);

COMMENT ON COLUMN public.sys_export_templates.db_name IS '数据库名称';

COMMENT ON COLUMN public.sys_export_templates.name IS '模板名称';

COMMENT ON COLUMN public.sys_export_templates.table_name IS '表名称';

COMMENT ON COLUMN public.sys_export_templates.template_id IS '模板标识';

COMMENT ON COLUMN public.sys_export_templates.sql IS '自定义导出SQL';

COMMENT ON COLUMN public.sys_export_templates.import_sql IS '自定义导入SQL';

COMMENT ON COLUMN public.sys_export_templates."limit" IS '导出限制';

COMMENT ON COLUMN public.sys_export_templates."order" IS '排序';

CREATE SEQUENCE public.sys_export_templates_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_export_templates_id_seq OWNED BY public.sys_export_templates.id;

CREATE TABLE public.sys_ignore_apis (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    path text,
    method text DEFAULT 'POST'::text
);

COMMENT ON COLUMN public.sys_ignore_apis.path IS 'api路径';

COMMENT ON COLUMN public.sys_ignore_apis.method IS '方法';

CREATE SEQUENCE public.sys_ignore_apis_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_ignore_apis_id_seq OWNED BY public.sys_ignore_apis.id;

CREATE TABLE public.sys_login_logs (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    username text,
    ip text,
    status boolean,
    error_message text,
    agent text,
    user_id bigint
);

COMMENT ON COLUMN public.sys_login_logs.username IS '用户名';

COMMENT ON COLUMN public.sys_login_logs.ip IS '请求ip';

COMMENT ON COLUMN public.sys_login_logs.status IS '登录状态';

COMMENT ON COLUMN public.sys_login_logs.error_message IS '错误信息';

COMMENT ON COLUMN public.sys_login_logs.agent IS '代理';

COMMENT ON COLUMN public.sys_login_logs.user_id IS '用户id';

CREATE SEQUENCE public.sys_login_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_login_logs_id_seq OWNED BY public.sys_login_logs.id;

CREATE TABLE public.sys_mcp_apis (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    mcp_id bigint NOT NULL,
    api_id bigint NOT NULL,
    command_name character varying(128),
    command_desc text,
    params_override text,
    api_brief character varying(255),
    response_override text,
    enabled boolean DEFAULT true NOT NULL,
    sort bigint DEFAULT 0 NOT NULL
);

COMMENT ON COLUMN public.sys_mcp_apis.mcp_id IS 'MCP ID';

COMMENT ON COLUMN public.sys_mcp_apis.api_id IS 'API ID';

COMMENT ON COLUMN public.sys_mcp_apis.command_name IS '工具名覆盖';

COMMENT ON COLUMN public.sys_mcp_apis.command_desc IS '工具说明覆盖';

COMMENT ON COLUMN public.sys_mcp_apis.params_override IS '参数定义覆盖JSON';

COMMENT ON COLUMN public.sys_mcp_apis.api_brief IS 'API简介覆盖';

COMMENT ON COLUMN public.sys_mcp_apis.response_override IS '返回字段定义覆盖JSON';

COMMENT ON COLUMN public.sys_mcp_apis.enabled IS '是否启用';

COMMENT ON COLUMN public.sys_mcp_apis.sort IS '排序';

CREATE SEQUENCE public.sys_mcp_apis_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_mcp_apis_id_seq OWNED BY public.sys_mcp_apis.id;

CREATE TABLE public.sys_mcps (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name character varying(128) NOT NULL,
    display_name character varying(128) NOT NULL,
    description text,
    status character varying(32) DEFAULT 'enabled'::character varying NOT NULL,
    version character varying(64) DEFAULT 'v1'::character varying NOT NULL,
    scenarios_json text
);

COMMENT ON COLUMN public.sys_mcps.name IS 'MCP唯一标识';

COMMENT ON COLUMN public.sys_mcps.display_name IS 'MCP展示名称';

COMMENT ON COLUMN public.sys_mcps.description IS 'MCP描述';

COMMENT ON COLUMN public.sys_mcps.status IS 'MCP状态';

COMMENT ON COLUMN public.sys_mcps.version IS 'MCP版本';

COMMENT ON COLUMN public.sys_mcps.scenarios_json IS 'tool间编排场景JSON';

CREATE SEQUENCE public.sys_mcps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_mcps_id_seq OWNED BY public.sys_mcps.id;

CREATE TABLE public.sys_operation_records (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    ip text,
    method text,
    path text,
    status bigint,
    latency_ms bigint,
    agent text,
    error_message text,
    body text,
    resp text,
    user_id bigint,
    request_id character varying(64),
    trace_id character varying(64),
    device_id character varying(64)
);

COMMENT ON COLUMN public.sys_operation_records.ip IS '请求ip';

COMMENT ON COLUMN public.sys_operation_records.method IS '请求方法';

COMMENT ON COLUMN public.sys_operation_records.path IS '请求路径';

COMMENT ON COLUMN public.sys_operation_records.status IS '请求状态';

COMMENT ON COLUMN public.sys_operation_records.latency_ms IS '延迟(毫秒)';

COMMENT ON COLUMN public.sys_operation_records.agent IS '代理';

COMMENT ON COLUMN public.sys_operation_records.error_message IS '错误信息';

COMMENT ON COLUMN public.sys_operation_records.body IS '请求Body';

COMMENT ON COLUMN public.sys_operation_records.resp IS '响应Body';

COMMENT ON COLUMN public.sys_operation_records.user_id IS '用户id';

COMMENT ON COLUMN public.sys_operation_records.request_id IS '请求ID';

COMMENT ON COLUMN public.sys_operation_records.trace_id IS '链路ID';

COMMENT ON COLUMN public.sys_operation_records.device_id IS '设备ID';

CREATE SEQUENCE public.sys_operation_records_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_operation_records_id_seq OWNED BY public.sys_operation_records.id;

CREATE TABLE public.sys_params (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    key text,
    value text,
    "desc" text
);

COMMENT ON COLUMN public.sys_params.name IS '参数名称';

COMMENT ON COLUMN public.sys_params.key IS '参数键';

COMMENT ON COLUMN public.sys_params.value IS '参数值';

COMMENT ON COLUMN public.sys_params."desc" IS '参数说明';

CREATE SEQUENCE public.sys_params_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_params_id_seq OWNED BY public.sys_params.id;

CREATE TABLE public.sys_positions (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    code text,
    sort bigint DEFAULT 0,
    status boolean DEFAULT true,
    remark text
);

COMMENT ON COLUMN public.sys_positions.name IS '岗位名称';

COMMENT ON COLUMN public.sys_positions.code IS '岗位编码';

COMMENT ON COLUMN public.sys_positions.sort IS '排序';

COMMENT ON COLUMN public.sys_positions.status IS '是否启用';

COMMENT ON COLUMN public.sys_positions.remark IS '备注';

CREATE SEQUENCE public.sys_positions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_positions_id_seq OWNED BY public.sys_positions.id;

CREATE TABLE public.sys_security_config (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    captcha_open bigint DEFAULT 0,
    captcha_timeout bigint DEFAULT 3600,
    key_long bigint DEFAULT 6,
    img_width bigint DEFAULT 240,
    img_height bigint DEFAULT 80,
    pwd_min_length bigint DEFAULT 8,
    pwd_require_upper boolean DEFAULT false,
    pwd_require_lower boolean DEFAULT false,
    pwd_require_digit boolean DEFAULT false,
    pwd_require_special boolean DEFAULT false,
    limit_enable boolean DEFAULT false,
    limit_window bigint DEFAULT 60,
    limit_count bigint DEFAULT 30,
    lock_enable boolean DEFAULT false,
    lock_threshold bigint DEFAULT 5,
    lock_duration bigint DEFAULT 30,
    pwd_expire_enable boolean DEFAULT false,
    pwd_expire_days bigint DEFAULT 90,
    force_new_user_change_password boolean DEFAULT false
);

COMMENT ON COLUMN public.sys_security_config.captcha_open IS '错误N次后出验证码 0=每次都要';

COMMENT ON COLUMN public.sys_security_config.captcha_timeout IS '防爆破计数缓存超时(秒)';

COMMENT ON COLUMN public.sys_security_config.key_long IS '验证码长度';

COMMENT ON COLUMN public.sys_security_config.img_width IS '验证码宽度';

COMMENT ON COLUMN public.sys_security_config.img_height IS '验证码高度';

COMMENT ON COLUMN public.sys_security_config.pwd_min_length IS '密码最小长度';

COMMENT ON COLUMN public.sys_security_config.pwd_require_upper IS '需大写字母';

COMMENT ON COLUMN public.sys_security_config.pwd_require_lower IS '需小写字母';

COMMENT ON COLUMN public.sys_security_config.pwd_require_digit IS '需数字';

COMMENT ON COLUMN public.sys_security_config.pwd_require_special IS '需特殊字符';

COMMENT ON COLUMN public.sys_security_config.limit_enable IS '是否开启限流';

COMMENT ON COLUMN public.sys_security_config.limit_window IS '限流窗口(秒)';

COMMENT ON COLUMN public.sys_security_config.limit_count IS '窗口内最大次数';

COMMENT ON COLUMN public.sys_security_config.lock_enable IS '是否开启失败锁定';

COMMENT ON COLUMN public.sys_security_config.lock_threshold IS '失败次数阈值';

COMMENT ON COLUMN public.sys_security_config.lock_duration IS '锁定时长(分钟)';

COMMENT ON COLUMN public.sys_security_config.pwd_expire_enable IS '是否开启密码过期';

COMMENT ON COLUMN public.sys_security_config.pwd_expire_days IS '密码有效天数';

COMMENT ON COLUMN public.sys_security_config.force_new_user_change_password IS '新用户首次登录是否强制改密';

CREATE SEQUENCE public.sys_security_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_security_config_id_seq OWNED BY public.sys_security_config.id;

CREATE TABLE public.sys_timed_task_logs (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    task_id bigint,
    task_name text,
    trigger_type text,
    started_at timestamp with time zone,
    finished_at timestamp with time zone,
    duration_ms bigint,
    status text,
    error_msg text,
    output text
);

COMMENT ON COLUMN public.sys_timed_task_logs.task_id IS '任务ID';

COMMENT ON COLUMN public.sys_timed_task_logs.task_name IS '任务名快照(任务删除后日志仍可读)';

COMMENT ON COLUMN public.sys_timed_task_logs.trigger_type IS '触发方式 auto/manual';

COMMENT ON COLUMN public.sys_timed_task_logs.started_at IS '开始时间';

COMMENT ON COLUMN public.sys_timed_task_logs.finished_at IS '结束时间';

COMMENT ON COLUMN public.sys_timed_task_logs.duration_ms IS '耗时毫秒';

COMMENT ON COLUMN public.sys_timed_task_logs.status IS '结果 success/fail/timeout';

COMMENT ON COLUMN public.sys_timed_task_logs.error_msg IS '错误信息(截断)';

COMMENT ON COLUMN public.sys_timed_task_logs.output IS '输出摘要(截断)';

CREATE SEQUENCE public.sys_timed_task_logs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_timed_task_logs_id_seq OWNED BY public.sys_timed_task_logs.id;

CREATE TABLE public.sys_timed_tasks (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    name text,
    description text,
    spec text,
    with_seconds boolean,
    executor_type text,
    method_name text,
    params jsonb,
    http_url text,
    http_method text,
    http_header jsonb,
    http_body text,
    http_allow_private boolean,
    enabled boolean
);

COMMENT ON COLUMN public.sys_timed_tasks.name IS '任务名';

COMMENT ON COLUMN public.sys_timed_tasks.description IS '任务说明(面板展示的提示)';

COMMENT ON COLUMN public.sys_timed_tasks.spec IS 'cron表达式(支持@daily等描述符)';

COMMENT ON COLUMN public.sys_timed_tasks.with_seconds IS '表达式是否含秒位';

COMMENT ON COLUMN public.sys_timed_tasks.executor_type IS '执行器类型 method/http';

COMMENT ON COLUMN public.sys_timed_tasks.method_name IS '已注册方法名(executor=method)';

COMMENT ON COLUMN public.sys_timed_tasks.params IS '方法自由JSON入参';

COMMENT ON COLUMN public.sys_timed_tasks.http_url IS 'HTTP回调地址(executor=http)';

COMMENT ON COLUMN public.sys_timed_tasks.http_method IS 'HTTP方法';

COMMENT ON COLUMN public.sys_timed_tasks.http_header IS 'HTTP自定义请求头(JSON对象)';

COMMENT ON COLUMN public.sys_timed_tasks.http_body IS 'HTTP请求体';

COMMENT ON COLUMN public.sys_timed_tasks.http_allow_private IS '允许访问内网/环回地址(默认禁止,SSRF防护)';

COMMENT ON COLUMN public.sys_timed_tasks.enabled IS '是否启用';

CREATE SEQUENCE public.sys_timed_tasks_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_timed_tasks_id_seq OWNED BY public.sys_timed_tasks.id;

CREATE TABLE public.sys_user_authority (
    sys_user_id bigint NOT NULL,
    sys_authority_authority_id bigint NOT NULL
);

COMMENT ON COLUMN public.sys_user_authority.sys_authority_authority_id IS '角色ID';

CREATE TABLE public.sys_user_departments (
    sys_user_id bigint NOT NULL,
    sys_department_id bigint NOT NULL
);

CREATE TABLE public.sys_user_positions (
    sys_user_id bigint NOT NULL,
    sys_position_id bigint NOT NULL
);

CREATE TABLE public.sys_users (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    uuid text,
    username text,
    password text,
    nick_name text DEFAULT '系统用户'::text,
    header_img text DEFAULT 'https://qmplusimg.henrongyi.top/gva_header.jpg'::text,
    authority_id bigint DEFAULT 888,
    dept_id bigint,
    phone text,
    email text,
    enable bigint DEFAULT 1,
    origin_setting jsonb,
    password_updated_at timestamp with time zone,
    must_change_password boolean DEFAULT false
);

COMMENT ON COLUMN public.sys_users.uuid IS '用户UUID';

COMMENT ON COLUMN public.sys_users.username IS '用户登录名';

COMMENT ON COLUMN public.sys_users.password IS '用户登录密码';

COMMENT ON COLUMN public.sys_users.nick_name IS '用户昵称';

COMMENT ON COLUMN public.sys_users.header_img IS '用户头像';

COMMENT ON COLUMN public.sys_users.authority_id IS '用户角色ID';

COMMENT ON COLUMN public.sys_users.dept_id IS '主部门ID(数据归属/盖章)';

COMMENT ON COLUMN public.sys_users.phone IS '用户手机号';

COMMENT ON COLUMN public.sys_users.email IS '用户邮箱';

COMMENT ON COLUMN public.sys_users.enable IS '用户是否被冻结 1正常 2冻结';

COMMENT ON COLUMN public.sys_users.origin_setting IS '配置';

COMMENT ON COLUMN public.sys_users.password_updated_at IS '密码最后修改时间';

COMMENT ON COLUMN public.sys_users.must_change_password IS '是否必须修改初始密码';

CREATE SEQUENCE public.sys_users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_users_id_seq OWNED BY public.sys_users.id;

CREATE TABLE public.sys_versions (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    version_name character varying(255),
    version_code character varying(100),
    description character varying(500),
    version_data text
);

COMMENT ON COLUMN public.sys_versions.version_name IS '版本名称';

COMMENT ON COLUMN public.sys_versions.version_code IS '版本号';

COMMENT ON COLUMN public.sys_versions.description IS '版本描述';

COMMENT ON COLUMN public.sys_versions.version_data IS '版本数据JSON';

CREATE SEQUENCE public.sys_versions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.sys_versions_id_seq OWNED BY public.sys_versions.id;

ALTER TABLE ONLY public.casbin_rule ALTER COLUMN id SET DEFAULT nextval('public.casbin_rule_id_seq'::regclass);

ALTER TABLE ONLY public.exa_customers ALTER COLUMN id SET DEFAULT nextval('public.exa_customers_id_seq'::regclass);

ALTER TABLE ONLY public.gva_announcements_info ALTER COLUMN id SET DEFAULT nextval('public.gva_announcements_info_id_seq'::regclass);

ALTER TABLE ONLY public.jwt_blacklists ALTER COLUMN id SET DEFAULT nextval('public.jwt_blacklists_id_seq'::regclass);

ALTER TABLE ONLY public.media_attachment_category ALTER COLUMN id SET DEFAULT nextval('public.media_attachment_category_id_seq'::regclass);

ALTER TABLE ONLY public.media_file_upload_and_downloads ALTER COLUMN id SET DEFAULT nextval('public.media_file_upload_and_downloads_id_seq'::regclass);

ALTER TABLE ONLY public.media_upload_chunks ALTER COLUMN id SET DEFAULT nextval('public.media_upload_chunks_id_seq'::regclass);

ALTER TABLE ONLY public.media_uploads ALTER COLUMN id SET DEFAULT nextval('public.media_uploads_id_seq'::regclass);

ALTER TABLE ONLY public.sys_api_tokens ALTER COLUMN id SET DEFAULT nextval('public.sys_api_tokens_id_seq'::regclass);

ALTER TABLE ONLY public.sys_apis ALTER COLUMN id SET DEFAULT nextval('public.sys_apis_id_seq'::regclass);

ALTER TABLE ONLY public.sys_authorities ALTER COLUMN authority_id SET DEFAULT nextval('public.sys_authorities_authority_id_seq'::regclass);

ALTER TABLE ONLY public.sys_auto_code_histories ALTER COLUMN id SET DEFAULT nextval('public.sys_auto_code_histories_id_seq'::regclass);

ALTER TABLE ONLY public.sys_auto_code_packages ALTER COLUMN id SET DEFAULT nextval('public.sys_auto_code_packages_id_seq'::regclass);

ALTER TABLE ONLY public.sys_base_menu_btns ALTER COLUMN id SET DEFAULT nextval('public.sys_base_menu_btns_id_seq'::regclass);

ALTER TABLE ONLY public.sys_base_menu_parameters ALTER COLUMN id SET DEFAULT nextval('public.sys_base_menu_parameters_id_seq'::regclass);

ALTER TABLE ONLY public.sys_base_menus ALTER COLUMN id SET DEFAULT nextval('public.sys_base_menus_id_seq'::regclass);

ALTER TABLE ONLY public.sys_cli_apis ALTER COLUMN id SET DEFAULT nextval('public.sys_cli_apis_id_seq'::regclass);

ALTER TABLE ONLY public.sys_clis ALTER COLUMN id SET DEFAULT nextval('public.sys_clis_id_seq'::regclass);

ALTER TABLE ONLY public.sys_data_access_logs ALTER COLUMN id SET DEFAULT nextval('public.sys_data_access_logs_id_seq'::regclass);

ALTER TABLE ONLY public.sys_departments ALTER COLUMN id SET DEFAULT nextval('public.sys_departments_id_seq'::regclass);

ALTER TABLE ONLY public.sys_dictionaries ALTER COLUMN id SET DEFAULT nextval('public.sys_dictionaries_id_seq'::regclass);

ALTER TABLE ONLY public.sys_dictionary_details ALTER COLUMN id SET DEFAULT nextval('public.sys_dictionary_details_id_seq'::regclass);

ALTER TABLE ONLY public.sys_error ALTER COLUMN id SET DEFAULT nextval('public.sys_error_id_seq'::regclass);

ALTER TABLE ONLY public.sys_export_template_condition ALTER COLUMN id SET DEFAULT nextval('public.sys_export_template_condition_id_seq'::regclass);

ALTER TABLE ONLY public.sys_export_template_join ALTER COLUMN id SET DEFAULT nextval('public.sys_export_template_join_id_seq'::regclass);

ALTER TABLE ONLY public.sys_export_templates ALTER COLUMN id SET DEFAULT nextval('public.sys_export_templates_id_seq'::regclass);

ALTER TABLE ONLY public.sys_ignore_apis ALTER COLUMN id SET DEFAULT nextval('public.sys_ignore_apis_id_seq'::regclass);

ALTER TABLE ONLY public.sys_login_logs ALTER COLUMN id SET DEFAULT nextval('public.sys_login_logs_id_seq'::regclass);

ALTER TABLE ONLY public.sys_mcp_apis ALTER COLUMN id SET DEFAULT nextval('public.sys_mcp_apis_id_seq'::regclass);

ALTER TABLE ONLY public.sys_mcps ALTER COLUMN id SET DEFAULT nextval('public.sys_mcps_id_seq'::regclass);

ALTER TABLE ONLY public.sys_operation_records ALTER COLUMN id SET DEFAULT nextval('public.sys_operation_records_id_seq'::regclass);

ALTER TABLE ONLY public.sys_params ALTER COLUMN id SET DEFAULT nextval('public.sys_params_id_seq'::regclass);

ALTER TABLE ONLY public.sys_positions ALTER COLUMN id SET DEFAULT nextval('public.sys_positions_id_seq'::regclass);

ALTER TABLE ONLY public.sys_security_config ALTER COLUMN id SET DEFAULT nextval('public.sys_security_config_id_seq'::regclass);

ALTER TABLE ONLY public.sys_timed_task_logs ALTER COLUMN id SET DEFAULT nextval('public.sys_timed_task_logs_id_seq'::regclass);

ALTER TABLE ONLY public.sys_timed_tasks ALTER COLUMN id SET DEFAULT nextval('public.sys_timed_tasks_id_seq'::regclass);

ALTER TABLE ONLY public.sys_users ALTER COLUMN id SET DEFAULT nextval('public.sys_users_id_seq'::regclass);

ALTER TABLE ONLY public.sys_versions ALTER COLUMN id SET DEFAULT nextval('public.sys_versions_id_seq'::regclass);

ALTER TABLE ONLY public.casbin_rule
    ADD CONSTRAINT casbin_rule_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.exa_customers
    ADD CONSTRAINT exa_customers_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.gva_announcements_info
    ADD CONSTRAINT gva_announcements_info_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.jwt_blacklists
    ADD CONSTRAINT jwt_blacklists_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.media_attachment_category
    ADD CONSTRAINT media_attachment_category_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.media_file_upload_and_downloads
    ADD CONSTRAINT media_file_upload_and_downloads_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.media_upload_chunks
    ADD CONSTRAINT media_upload_chunks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.media_uploads
    ADD CONSTRAINT media_uploads_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_api_tokens
    ADD CONSTRAINT sys_api_tokens_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_apis
    ADD CONSTRAINT sys_apis_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_authority_menus
    ADD CONSTRAINT sys_authority_menus_pkey PRIMARY KEY (sys_base_menu_id, sys_authority_authority_id);

ALTER TABLE ONLY public.sys_auto_code_histories
    ADD CONSTRAINT sys_auto_code_histories_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_auto_code_packages
    ADD CONSTRAINT sys_auto_code_packages_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_base_menu_btns
    ADD CONSTRAINT sys_base_menu_btns_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_base_menu_parameters
    ADD CONSTRAINT sys_base_menu_parameters_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_base_menus
    ADD CONSTRAINT sys_base_menus_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_cli_apis
    ADD CONSTRAINT sys_cli_apis_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_clis
    ADD CONSTRAINT sys_clis_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_data_access_logs
    ADD CONSTRAINT sys_data_access_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_departments
    ADD CONSTRAINT sys_departments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_dictionaries
    ADD CONSTRAINT sys_dictionaries_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_dictionary_details
    ADD CONSTRAINT sys_dictionary_details_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_error
    ADD CONSTRAINT sys_error_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_export_template_condition
    ADD CONSTRAINT sys_export_template_condition_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_export_template_join
    ADD CONSTRAINT sys_export_template_join_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_export_templates
    ADD CONSTRAINT sys_export_templates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_ignore_apis
    ADD CONSTRAINT sys_ignore_apis_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_login_logs
    ADD CONSTRAINT sys_login_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_mcp_apis
    ADD CONSTRAINT sys_mcp_apis_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_mcps
    ADD CONSTRAINT sys_mcps_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_operation_records
    ADD CONSTRAINT sys_operation_records_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_params
    ADD CONSTRAINT sys_params_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_positions
    ADD CONSTRAINT sys_positions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_security_config
    ADD CONSTRAINT sys_security_config_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_timed_task_logs
    ADD CONSTRAINT sys_timed_task_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_timed_tasks
    ADD CONSTRAINT sys_timed_tasks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_user_authority
    ADD CONSTRAINT sys_user_authority_pkey PRIMARY KEY (sys_user_id, sys_authority_authority_id);

ALTER TABLE ONLY public.sys_user_departments
    ADD CONSTRAINT sys_user_departments_pkey PRIMARY KEY (sys_user_id, sys_department_id);

ALTER TABLE ONLY public.sys_user_positions
    ADD CONSTRAINT sys_user_positions_pkey PRIMARY KEY (sys_user_id, sys_position_id);

ALTER TABLE ONLY public.sys_users
    ADD CONSTRAINT sys_users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_versions
    ADD CONSTRAINT sys_versions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.sys_authorities
    ADD CONSTRAINT uni_sys_authorities_authority_id PRIMARY KEY (authority_id);

CREATE UNIQUE INDEX idx_cli_api ON public.sys_cli_apis USING btree (cli_id, api_id);

CREATE INDEX idx_exa_customers_deleted_at ON public.exa_customers USING btree (deleted_at);

CREATE INDEX idx_gva_announcements_info_deleted_at ON public.gva_announcements_info USING btree (deleted_at);

CREATE INDEX idx_jwt_blacklists_deleted_at ON public.jwt_blacklists USING btree (deleted_at);

CREATE UNIQUE INDEX idx_mcp_api ON public.sys_mcp_apis USING btree (mcp_id, api_id);

CREATE INDEX idx_media_attachment_category_deleted_at ON public.media_attachment_category USING btree (deleted_at);

CREATE INDEX idx_media_file_upload_and_downloads_deleted_at ON public.media_file_upload_and_downloads USING btree (deleted_at);

CREATE INDEX idx_media_file_upload_and_downloads_md5 ON public.media_file_upload_and_downloads USING btree (md5);

CREATE INDEX idx_media_file_upload_and_downloads_user_id ON public.media_file_upload_and_downloads USING btree (user_id);

CREATE INDEX idx_media_upload_chunks_deleted_at ON public.media_upload_chunks USING btree (deleted_at);

CREATE INDEX idx_media_uploads_deleted_at ON public.media_uploads USING btree (deleted_at);

CREATE INDEX idx_media_uploads_file_hash ON public.media_uploads USING btree (file_hash);

CREATE INDEX idx_media_uploads_user_id ON public.media_uploads USING btree (user_id);

CREATE INDEX idx_sys_api_tokens_deleted_at ON public.sys_api_tokens USING btree (deleted_at);

CREATE INDEX idx_sys_apis_deleted_at ON public.sys_apis USING btree (deleted_at);

CREATE INDEX idx_sys_authority_departments_sys_authority_authority_id ON public.sys_authority_departments USING btree (sys_authority_authority_id);

CREATE INDEX idx_sys_auto_code_histories_deleted_at ON public.sys_auto_code_histories USING btree (deleted_at);

CREATE INDEX idx_sys_auto_code_packages_deleted_at ON public.sys_auto_code_packages USING btree (deleted_at);

CREATE INDEX idx_sys_base_menu_btns_deleted_at ON public.sys_base_menu_btns USING btree (deleted_at);

CREATE INDEX idx_sys_base_menu_parameters_deleted_at ON public.sys_base_menu_parameters USING btree (deleted_at);

CREATE INDEX idx_sys_base_menus_deleted_at ON public.sys_base_menus USING btree (deleted_at);

CREATE INDEX idx_sys_cli_apis_deleted_at ON public.sys_cli_apis USING btree (deleted_at);

CREATE INDEX idx_sys_clis_deleted_at ON public.sys_clis USING btree (deleted_at);

CREATE UNIQUE INDEX idx_sys_clis_name ON public.sys_clis USING btree (name);

CREATE INDEX idx_sys_data_access_logs_deleted_at ON public.sys_data_access_logs USING btree (deleted_at);

CREATE INDEX idx_sys_data_access_logs_event_type ON public.sys_data_access_logs USING btree (event_type);

CREATE INDEX idx_sys_data_access_logs_target_table ON public.sys_data_access_logs USING btree (target_table);

CREATE INDEX idx_sys_departments_deleted_at ON public.sys_departments USING btree (deleted_at);

CREATE INDEX idx_sys_departments_name ON public.sys_departments USING btree (name);

CREATE INDEX idx_sys_dictionaries_deleted_at ON public.sys_dictionaries USING btree (deleted_at);

CREATE INDEX idx_sys_dictionary_details_deleted_at ON public.sys_dictionary_details USING btree (deleted_at);

CREATE INDEX idx_sys_error_deleted_at ON public.sys_error USING btree (deleted_at);

CREATE INDEX idx_sys_error_request_id ON public.sys_error USING btree (request_id);

CREATE INDEX idx_sys_error_trace_id ON public.sys_error USING btree (trace_id);

CREATE INDEX idx_sys_export_template_condition_deleted_at ON public.sys_export_template_condition USING btree (deleted_at);

CREATE INDEX idx_sys_export_template_join_deleted_at ON public.sys_export_template_join USING btree (deleted_at);

CREATE INDEX idx_sys_export_templates_deleted_at ON public.sys_export_templates USING btree (deleted_at);

CREATE INDEX idx_sys_ignore_apis_deleted_at ON public.sys_ignore_apis USING btree (deleted_at);

CREATE INDEX idx_sys_login_logs_deleted_at ON public.sys_login_logs USING btree (deleted_at);

CREATE INDEX idx_sys_mcp_apis_deleted_at ON public.sys_mcp_apis USING btree (deleted_at);

CREATE INDEX idx_sys_mcps_deleted_at ON public.sys_mcps USING btree (deleted_at);

CREATE UNIQUE INDEX idx_sys_mcps_name ON public.sys_mcps USING btree (name);

CREATE INDEX idx_sys_operation_records_deleted_at ON public.sys_operation_records USING btree (deleted_at);

CREATE INDEX idx_sys_operation_records_request_id ON public.sys_operation_records USING btree (request_id);

CREATE INDEX idx_sys_operation_records_trace_id ON public.sys_operation_records USING btree (trace_id);

CREATE INDEX idx_sys_params_deleted_at ON public.sys_params USING btree (deleted_at);

CREATE INDEX idx_sys_positions_deleted_at ON public.sys_positions USING btree (deleted_at);

CREATE INDEX idx_sys_positions_name ON public.sys_positions USING btree (name);

CREATE INDEX idx_sys_security_config_deleted_at ON public.sys_security_config USING btree (deleted_at);

CREATE INDEX idx_sys_timed_task_logs_deleted_at ON public.sys_timed_task_logs USING btree (deleted_at);

CREATE INDEX idx_sys_timed_task_logs_task_id ON public.sys_timed_task_logs USING btree (task_id);

CREATE INDEX idx_sys_timed_tasks_deleted_at ON public.sys_timed_tasks USING btree (deleted_at);

CREATE INDEX idx_sys_timed_tasks_name ON public.sys_timed_tasks USING btree (name);

CREATE INDEX idx_sys_users_deleted_at ON public.sys_users USING btree (deleted_at);

CREATE INDEX idx_sys_users_username ON public.sys_users USING btree (username);

CREATE INDEX idx_sys_users_uuid ON public.sys_users USING btree (uuid);

CREATE INDEX idx_sys_versions_deleted_at ON public.sys_versions USING btree (deleted_at);

CREATE UNIQUE INDEX idx_upload_chunk ON public.media_upload_chunks USING btree (upload_id, chunk_index);

-- +goose StatementEnd

-- +goose Down
-- Moonbook migrations are forward-only.
-- +goose StatementBegin
DO $$
BEGIN
    RAISE EXCEPTION 'Moonbook migrations are forward-only; create a higher corrective migration';
END
$$;
-- +goose StatementEnd

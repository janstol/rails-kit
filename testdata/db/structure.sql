--
-- PostgreSQL database dump
--

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: ar_internal_metadata; Type: TABLE
--

CREATE TABLE ar_internal_metadata (
    key character varying NOT NULL,
    value character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

--
-- Name: comments; Type: TABLE
--

CREATE TABLE comments (
    id bigint NOT NULL,
    post_id bigint NOT NULL,
    user_id bigint NOT NULL,
    body text NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

--
-- Name: comments_id_seq; Type: SEQUENCE
--

CREATE SEQUENCE comments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE comments_id_seq OWNED BY comments.id;

--
-- Name: posts; Type: TABLE
--

CREATE TABLE posts (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    title character varying NOT NULL,
    body text,
    status character varying DEFAULT 'draft'::character varying NOT NULL,
    published_at timestamp(6) without time zone,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

--
-- Name: posts_id_seq; Type: SEQUENCE
--

CREATE SEQUENCE posts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE posts_id_seq OWNED BY posts.id;

--
-- Name: posts_tags; Type: TABLE
--

CREATE TABLE posts_tags (
    post_id bigint NOT NULL,
    tag_id bigint NOT NULL
);

--
-- Name: schema_migrations; Type: TABLE
--

CREATE TABLE schema_migrations (
    version character varying NOT NULL
);

--
-- Name: tags; Type: TABLE
--

CREATE TABLE tags (
    id bigint NOT NULL,
    name character varying NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

--
-- Name: tags_id_seq; Type: SEQUENCE
--

CREATE SEQUENCE tags_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE tags_id_seq OWNED BY tags.id;

--
-- Name: users; Type: TABLE
--

CREATE TABLE users (
    id bigint NOT NULL,
    email character varying NOT NULL,
    name character varying NOT NULL,
    role character varying DEFAULT 'member'::character varying NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

--
-- Name: users_id_seq; Type: SEQUENCE
--

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE users_id_seq OWNED BY users.id;

--
-- Name: comments id; Type: DEFAULT
--

ALTER TABLE ONLY comments ALTER COLUMN id SET DEFAULT nextval('comments_id_seq'::regclass);

--
-- Name: posts id; Type: DEFAULT
--

ALTER TABLE ONLY posts ALTER COLUMN id SET DEFAULT nextval('posts_id_seq'::regclass);

--
-- Name: tags id; Type: DEFAULT
--

ALTER TABLE ONLY tags ALTER COLUMN id SET DEFAULT nextval('tags_id_seq'::regclass);

--
-- Name: users id; Type: DEFAULT
--

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);

--
-- Name: ar_internal_metadata ar_internal_metadata_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY ar_internal_metadata
    ADD CONSTRAINT ar_internal_metadata_pkey PRIMARY KEY (key);

--
-- Name: comments comments_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY comments
    ADD CONSTRAINT comments_pkey PRIMARY KEY (id);

--
-- Name: posts posts_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY posts
    ADD CONSTRAINT posts_pkey PRIMARY KEY (id);

--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);

--
-- Name: tags tags_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);

--
-- Name: users users_pkey; Type: CONSTRAINT
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

--
-- Name: index_comments_on_post_id; Type: INDEX
--

CREATE INDEX index_comments_on_post_id ON comments USING btree (post_id);

--
-- Name: index_comments_on_post_id_and_user_id; Type: INDEX
--

CREATE INDEX index_comments_on_post_id_and_user_id ON comments USING btree (post_id, user_id);

--
-- Name: index_comments_on_user_id; Type: INDEX
--

CREATE INDEX index_comments_on_user_id ON comments USING btree (user_id);

--
-- Name: index_posts_on_status; Type: INDEX
--

CREATE INDEX index_posts_on_status ON posts USING btree (status);

--
-- Name: index_posts_on_user_id; Type: INDEX
--

CREATE INDEX index_posts_on_user_id ON posts USING btree (user_id);

--
-- Name: index_posts_tags_on_post_id_and_tag_id; Type: INDEX
--

CREATE UNIQUE INDEX index_posts_tags_on_post_id_and_tag_id ON posts_tags USING btree (post_id, tag_id);

--
-- Name: index_users_on_email; Type: INDEX
--

CREATE UNIQUE INDEX index_users_on_email ON users USING btree (email);

--
-- Name: comments fk_rails_comments_posts; Type: FK CONSTRAINT
--

ALTER TABLE ONLY comments
    ADD CONSTRAINT fk_rails_comments_posts FOREIGN KEY (post_id) REFERENCES posts(id);

--
-- Name: comments fk_rails_comments_users; Type: FK CONSTRAINT
--

ALTER TABLE ONLY comments
    ADD CONSTRAINT fk_rails_comments_users FOREIGN KEY (user_id) REFERENCES users(id);

--
-- Name: posts fk_rails_posts_users; Type: FK CONSTRAINT
--

ALTER TABLE ONLY posts
    ADD CONSTRAINT fk_rails_posts_users FOREIGN KEY (user_id) REFERENCES users(id);

--
-- PostgreSQL database dump complete
--

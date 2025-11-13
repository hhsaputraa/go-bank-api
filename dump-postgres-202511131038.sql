--
-- PostgreSQL database dump
--

-- Dumped from database version 17.0
-- Dumped by pg_dump version 17.0

-- Started on 2025-11-13 10:38:04

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP DATABASE postgres;
--
-- TOC entry 4953 (class 1262 OID 5)
-- Name: postgres; Type: DATABASE; Schema: -; Owner: -
--

CREATE DATABASE postgres WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE_PROVIDER = libc LOCALE = 'Indonesian_Indonesia.1252';


\connect postgres

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- TOC entry 4954 (class 0 OID 0)
-- Dependencies: 4953
-- Name: DATABASE postgres; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON DATABASE postgres IS 'default administrative connection database';


--
-- TOC entry 6 (class 2615 OID 107379)
-- Name: bpr_supra; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA bpr_supra;


--
-- TOC entry 5 (class 2615 OID 2200)
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- TOC entry 4955 (class 0 OID 0)
-- Dependencies: 5
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


--
-- TOC entry 883 (class 1247 OID 107493)
-- Name: tipe_dk; Type: TYPE; Schema: bpr_supra; Owner: -
--

CREATE TYPE bpr_supra.tipe_dk AS ENUM (
    'DEBIT',
    'KREDIT'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- TOC entry 231 (class 1259 OID 107498)
-- Name: jurnal_transaksi; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.jurnal_transaksi (
    id_jurnal bigint NOT NULL,
    id_transaksi bigint NOT NULL,
    id_rekening character varying(20) NOT NULL,
    tipe_dk bpr_supra.tipe_dk NOT NULL,
    jumlah numeric(19,2) NOT NULL,
    CONSTRAINT jurnal_transaksi_jumlah_check CHECK ((jumlah > (0)::numeric))
);


--
-- TOC entry 230 (class 1259 OID 107497)
-- Name: jurnal_transaksi_id_jurnal_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.jurnal_transaksi_id_jurnal_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4956 (class 0 OID 0)
-- Dependencies: 230
-- Name: jurnal_transaksi_id_jurnal_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.jurnal_transaksi_id_jurnal_seq OWNED BY bpr_supra.jurnal_transaksi.id_jurnal;


--
-- TOC entry 221 (class 1259 OID 107420)
-- Name: master_jenis_rekening; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.master_jenis_rekening (
    id_jenis_rekening smallint NOT NULL,
    nama_jenis character varying(50) NOT NULL
);


--
-- TOC entry 220 (class 1259 OID 107419)
-- Name: master_jenis_rekening_id_jenis_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.master_jenis_rekening_id_jenis_seq
    AS smallint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4957 (class 0 OID 0)
-- Dependencies: 220
-- Name: master_jenis_rekening_id_jenis_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.master_jenis_rekening_id_jenis_seq OWNED BY bpr_supra.master_jenis_rekening.id_jenis_rekening;


--
-- TOC entry 223 (class 1259 OID 107429)
-- Name: master_status_rekening; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.master_status_rekening (
    id_status_rekening smallint NOT NULL,
    nama_status character varying(50) NOT NULL
);


--
-- TOC entry 222 (class 1259 OID 107428)
-- Name: master_status_rekening_id_status_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.master_status_rekening_id_status_seq
    AS smallint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4958 (class 0 OID 0)
-- Dependencies: 222
-- Name: master_status_rekening_id_status_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.master_status_rekening_id_status_seq OWNED BY bpr_supra.master_status_rekening.id_status_rekening;


--
-- TOC entry 219 (class 1259 OID 107411)
-- Name: master_tipe_nasabah; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.master_tipe_nasabah (
    id_tipe_nasabah smallint NOT NULL,
    nama_tipe character varying(50) NOT NULL
);


--
-- TOC entry 218 (class 1259 OID 107410)
-- Name: master_tipe_nasabah_id_tipe_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.master_tipe_nasabah_id_tipe_seq
    AS smallint
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4959 (class 0 OID 0)
-- Dependencies: 218
-- Name: master_tipe_nasabah_id_tipe_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.master_tipe_nasabah_id_tipe_seq OWNED BY bpr_supra.master_tipe_nasabah.id_tipe_nasabah;


--
-- TOC entry 225 (class 1259 OID 107438)
-- Name: master_tipe_transaksi; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.master_tipe_transaksi (
    id_tipe_transaksi integer NOT NULL,
    kode_transaksi character varying(10) NOT NULL,
    nama_transaksi character varying(100) NOT NULL
);


--
-- TOC entry 224 (class 1259 OID 107437)
-- Name: master_tipe_transaksi_id_tipe_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.master_tipe_transaksi_id_tipe_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4960 (class 0 OID 0)
-- Dependencies: 224
-- Name: master_tipe_transaksi_id_tipe_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.master_tipe_transaksi_id_tipe_seq OWNED BY bpr_supra.master_tipe_transaksi.id_tipe_transaksi;


--
-- TOC entry 226 (class 1259 OID 107446)
-- Name: nasabah; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.nasabah (
    id_nasabah character varying(20) NOT NULL,
    nama_lengkap character varying(150) NOT NULL,
    alamat text,
    tanggal_lahir date NOT NULL,
    id_tipe_nasabah smallint NOT NULL
);


--
-- TOC entry 233 (class 1259 OID 107554)
-- Name: rag_sql_examples; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.rag_sql_examples (
    id integer NOT NULL,
    prompt_example text NOT NULL,
    sql_example text NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);


--
-- TOC entry 4961 (class 0 OID 0)
-- Dependencies: 233
-- Name: TABLE rag_sql_examples; Type: COMMENT; Schema: bpr_supra; Owner: -
--

COMMENT ON TABLE bpr_supra.rag_sql_examples IS 'Tabel ini berisi contoh-contoh (contekan) pertanyaan user dan jawaban SQL yang benar untuk keperluan training RAG (Retrieval-Augmented Generation) AI.';


--
-- TOC entry 232 (class 1259 OID 107553)
-- Name: rag_sql_examples_id_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.rag_sql_examples_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4962 (class 0 OID 0)
-- Dependencies: 232
-- Name: rag_sql_examples_id_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.rag_sql_examples_id_seq OWNED BY bpr_supra.rag_sql_examples.id;


--
-- TOC entry 227 (class 1259 OID 107458)
-- Name: rekening; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.rekening (
    id_rekening character varying(20) NOT NULL,
    id_nasabah character varying(20) NOT NULL,
    id_jenis_rekening smallint NOT NULL,
    id_status_rekening smallint NOT NULL,
    tanggal_buka date DEFAULT CURRENT_DATE NOT NULL
);


--
-- TOC entry 229 (class 1259 OID 107480)
-- Name: transaksi; Type: TABLE; Schema: bpr_supra; Owner: -
--

CREATE TABLE bpr_supra.transaksi (
    id_transaksi bigint NOT NULL,
    waktu_transaksi timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    id_tipe_transaksi integer NOT NULL,
    deskripsi character varying(255)
);


--
-- TOC entry 228 (class 1259 OID 107479)
-- Name: transaksi_id_transaksi_seq; Type: SEQUENCE; Schema: bpr_supra; Owner: -
--

CREATE SEQUENCE bpr_supra.transaksi_id_transaksi_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- TOC entry 4963 (class 0 OID 0)
-- Dependencies: 228
-- Name: transaksi_id_transaksi_seq; Type: SEQUENCE OWNED BY; Schema: bpr_supra; Owner: -
--

ALTER SEQUENCE bpr_supra.transaksi_id_transaksi_seq OWNED BY bpr_supra.transaksi.id_transaksi;


--
-- TOC entry 4746 (class 2604 OID 107501)
-- Name: jurnal_transaksi id_jurnal; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.jurnal_transaksi ALTER COLUMN id_jurnal SET DEFAULT nextval('bpr_supra.jurnal_transaksi_id_jurnal_seq'::regclass);


--
-- TOC entry 4740 (class 2604 OID 107423)
-- Name: master_jenis_rekening id_jenis_rekening; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_jenis_rekening ALTER COLUMN id_jenis_rekening SET DEFAULT nextval('bpr_supra.master_jenis_rekening_id_jenis_seq'::regclass);


--
-- TOC entry 4741 (class 2604 OID 107432)
-- Name: master_status_rekening id_status_rekening; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_status_rekening ALTER COLUMN id_status_rekening SET DEFAULT nextval('bpr_supra.master_status_rekening_id_status_seq'::regclass);


--
-- TOC entry 4739 (class 2604 OID 107414)
-- Name: master_tipe_nasabah id_tipe_nasabah; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_nasabah ALTER COLUMN id_tipe_nasabah SET DEFAULT nextval('bpr_supra.master_tipe_nasabah_id_tipe_seq'::regclass);


--
-- TOC entry 4742 (class 2604 OID 107441)
-- Name: master_tipe_transaksi id_tipe_transaksi; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_transaksi ALTER COLUMN id_tipe_transaksi SET DEFAULT nextval('bpr_supra.master_tipe_transaksi_id_tipe_seq'::regclass);


--
-- TOC entry 4747 (class 2604 OID 107557)
-- Name: rag_sql_examples id; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rag_sql_examples ALTER COLUMN id SET DEFAULT nextval('bpr_supra.rag_sql_examples_id_seq'::regclass);


--
-- TOC entry 4744 (class 2604 OID 107483)
-- Name: transaksi id_transaksi; Type: DEFAULT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.transaksi ALTER COLUMN id_transaksi SET DEFAULT nextval('bpr_supra.transaksi_id_transaksi_seq'::regclass);


--
-- TOC entry 4945 (class 0 OID 107498)
-- Dependencies: 231
-- Data for Name: jurnal_transaksi; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.jurnal_transaksi VALUES (1, 1, '110000001', 'KREDIT', 1000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (2, 2, '110000002', 'DEBIT', 500000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (3, 3, '110000001', 'DEBIT', 250000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (4, 3, '110000002', 'KREDIT', 250000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (5, 4, '110000001', 'DEBIT', 15000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (6, 5, '220000001', 'KREDIT', 50000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (7, 6, '110000002', 'DEBIT', 15000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (8, 7, '220000002', 'KREDIT', 100000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (10, 9, '140000001', 'KREDIT', 5000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (11, 10, '110000002', 'DEBIT', 300000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (12, 11, '220000001', 'DEBIT', 25000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (13, 12, '110000001', 'KREDIT', 8000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (14, 13, '110000002', 'DEBIT', 450000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (15, 14, '220000001', 'KREDIT', 120000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (16, 15, '220000001', 'DEBIT', 75000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (17, 16, '220000002', 'DEBIT', 50000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (18, 17, '220000001', 'DEBIT', 25000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (19, 18, '220000002', 'DEBIT', 25000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (20, 19, '140000001', 'DEBIT', 5000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (21, 20, '220000001', 'DEBIT', 15000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (22, 20, '220000002', 'KREDIT', 15000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (23, 21, '140000001', 'KREDIT', 300.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (24, 22, '140000001', 'DEBIT', 150.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (25, 23, '110000002', 'KREDIT', 850000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (26, 24, '130000001', 'DEBIT', 20000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (27, 25, '110000001', 'DEBIT', 1000000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (9, 8, '110000001', 'DEBIT', 500000.00);
INSERT INTO bpr_supra.jurnal_transaksi VALUES (29, 26, '110000002', 'KREDIT', 2000000.00);


--
-- TOC entry 4935 (class 0 OID 107420)
-- Dependencies: 221
-- Data for Name: master_jenis_rekening; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.master_jenis_rekening VALUES (1, 'tabungan');
INSERT INTO bpr_supra.master_jenis_rekening VALUES (2, 'giro');
INSERT INTO bpr_supra.master_jenis_rekening VALUES (3, 'deposito berjangka');
INSERT INTO bpr_supra.master_jenis_rekening VALUES (4, 'rekening valas');


--
-- TOC entry 4937 (class 0 OID 107429)
-- Dependencies: 223
-- Data for Name: master_status_rekening; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.master_status_rekening VALUES (1, 'aktif');
INSERT INTO bpr_supra.master_status_rekening VALUES (2, 'tidak_aktif');
INSERT INTO bpr_supra.master_status_rekening VALUES (3, 'ditutup');
INSERT INTO bpr_supra.master_status_rekening VALUES (4, 'dormant');


--
-- TOC entry 4933 (class 0 OID 107411)
-- Dependencies: 219
-- Data for Name: master_tipe_nasabah; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.master_tipe_nasabah VALUES (1, 'perseorangan');
INSERT INTO bpr_supra.master_tipe_nasabah VALUES (2, 'perusahaan');
INSERT INTO bpr_supra.master_tipe_nasabah VALUES (3, 'intitusi non profit');


--
-- TOC entry 4939 (class 0 OID 107438)
-- Dependencies: 225
-- Data for Name: master_tipe_transaksi; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.master_tipe_transaksi VALUES (1, 'DEP', 'Setoran Tunai (Deposit)');
INSERT INTO bpr_supra.master_tipe_transaksi VALUES (2, 'WDL', 'Penarikan Tunai (Withdrawal)');
INSERT INTO bpr_supra.master_tipe_transaksi VALUES (3, 'TRF_IN', 'Transfer Masuk (Inbound Transfer)');
INSERT INTO bpr_supra.master_tipe_transaksi VALUES (4, 'TRF_OUT', 'Transfer Keluar (Outbound Transfer)');
INSERT INTO bpr_supra.master_tipe_transaksi VALUES (5, 'ADM_FEE', 'Biaya Administrasi Bulanan');


--
-- TOC entry 4940 (class 0 OID 107446)
-- Dependencies: 226
-- Data for Name: nasabah; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.nasabah VALUES ('CIF00001', 'Budi Santoso', 'Jl. Merdeka No. 10, Jakarta Pusat', '1985-05-15', 1);
INSERT INTO bpr_supra.nasabah VALUES ('CIF00002', 'Citra Lestari', 'Jl. Pahlawan No. 25, Bandung', '1992-11-30', 1);
INSERT INTO bpr_supra.nasabah VALUES ('CIF00003', 'Ahmad Wijaya', 'Jl. Gajah Mada No. 101, Semarang', '1978-01-20', 1);
INSERT INTO bpr_supra.nasabah VALUES ('CIF00004', 'Dewi Anggraini', 'Jl. Kenanga No. 8, Surabaya', '1995-03-12', 1);
INSERT INTO bpr_supra.nasabah VALUES ('CIF10001', 'PT. Maju Jaya Abadi', 'Kawasan Industri Cikarang Blok A5, Bekasi', '2010-07-01', 2);
INSERT INTO bpr_supra.nasabah VALUES ('CIF00005', 'Eka Saputra', NULL, '1990-09-05', 1);
INSERT INTO bpr_supra.nasabah VALUES ('CIF10002', 'CV. Sinar Terang', 'Jl. Industri Raya No. 33, Medan', '2015-02-28', 2);


--
-- TOC entry 4947 (class 0 OID 107554)
-- Dependencies: 233
-- Data for Name: rag_sql_examples; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.rag_sql_examples VALUES (1, '-- Pertanyaan: "tampilkan semua nasabah"', 'SELECT id_nasabah, nama_lengkap, alamat, tanggal_lahir FROM nasabah;', '2025-11-11 09:33:50.371315+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (2, '-- Pertanyaan: "ada berapa nasabah?"', 'SELECT COUNT(*) AS jumlah_nasabah FROM nasabah;', '2025-11-11 09:33:50.371315+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (3, '-- Pertanyaan: "siapa nasabah dengan cif CIF00001?"', 'SELECT id_nasabah, nama_lengkap FROM nasabah WHERE id_nasabah = ''CIF00001'';', '2025-11-11 09:33:50.371315+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (4, '-- Pertanyaan: "nasabah atau penabung dengan nama a"', 'SELECT id_nasabah, nama_lengkap FROM nasabah WHERE nama_lengkap ILIKE ''a%'';', '2025-11-11 09:39:44.696937+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (5, '-- Pertanyaan: "siapa nasabah dengan penabung terbanyak?"', 'SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo
FROM jurnal_transaksi jt JOIN rekening r ON jt.id_rekening = r.id_rekening JOIN nasabah n ON r.id_nasabah = n.id_nasabah
GROUP BY n.id_nasabah, n.nama_lengkap ORDER BY total_saldo DESC LIMIT 1;', '2025-11-11 09:42:46.333887+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (6, '-- Pertanyaan: "total nasabah saat ini?"', 'SELECT COUNT(*) AS jumlah_nasabah FROM nasabah;', '2025-11-11 10:49:57.419827+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (7, '-- Pertanyaan: "tolong hitung semua nasabah yang terdaftar"', 'SELECT COUNT(*) AS jumlah_nasabah FROM nasabah;', '2025-11-11 10:49:57.419827+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (8, '-- Pertanyaan: "daftar semua nasabah dan alamatnya"', 'SELECT id_nasabah, nama_lengkap, alamat FROM nasabah;', '2025-11-11 10:49:57.419827+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (9, '-- Pertanyaan: "cari nasabah yang namanya ada kata budi"', 'SELECT id_nasabah, nama_lengkap, alamat FROM nasabah WHERE nama_lengkap ILIKE ''%budi%'';', '2025-11-11 10:49:57.419827+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (10, '-- Pertanyaan: "tampilkan semua transaksi hari ini"', 'SELECT t.waktu_transaksi, t.deskripsi, jt.jumlah, jt.tipe_dk
     FROM transaksi t
     JOIN jurnal_transaksi jt ON t.id_transaksi = jt.id_transaksi
     WHERE t.waktu_transaksi >= CURRENT_DATE 
       AND t.waktu_transaksi < (CURRENT_DATE + INTERVAL ''1 day'');', '2025-11-11 10:51:12.972726+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (11, '-- Pertanyaan: "ada berapa transaksi minggu ini?"', 'SELECT COUNT(DISTINCT id_transaksi) AS jumlah_transaksi
     FROM transaksi
     WHERE waktu_transaksi >= DATE_TRUNC(''week'', CURRENT_DATE);', '2025-11-11 10:51:12.972726+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (12, '-- Pertanyaan: "tampilkan transaksi antara 1 januari 2024 sampai 15 januari 2024"', 'SELECT t.id_transaksi, t.waktu_transaksi, t.deskripsi
     FROM transaksi t
     WHERE t.waktu_transaksi >= ''2024-01-01'' AND t.waktu_transaksi <= ''2024-01-15'';', '2025-11-11 10:51:12.972726+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (13, '-- Pertanyaan: "berapa rata-rata saldo semua nasabah?"', 'WITH saldo_nasabah AS (
        SELECT 
            r.id_nasabah, 
            SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS saldo
        FROM jurnal_transaksi jt
        JOIN rekening r ON jt.id_rekening = r.id_rekening
        GROUP BY r.id_nasabah
    )
    SELECT AVG(saldo) AS rata_rata_saldo FROM saldo_nasabah;', '2025-11-11 10:51:21.54458+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (14, '-- Pertanyaan: "berapa nilai transaksi terbesar yang pernah terjadi?"', 'SELECT MAX(jumlah) AS transaksi_terbesar FROM jurnal_transaksi;', '2025-11-11 10:51:21.54458+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (15, '-- Pertanyaan: "tampilkan 5 nasabah dengan saldo paling sedikit"', 'SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo
     FROM jurnal_transaksi jt 
     JOIN rekening r ON jt.id_rekening = r.id_rekening 
     JOIN nasabah n ON r.id_nasabah = n.id_nasabah
     GROUP BY n.id_nasabah, n.nama_lengkap 
     ORDER BY total_saldo ASC 
     LIMIT 5;', '2025-11-11 10:51:21.54458+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (16, '-- Pertanyaan: "berapa rata-rata saldo semua nasabah?"', 'WITH saldo_nasabah AS (
        SELECT 
            r.id_nasabah, 
            SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS saldo
        FROM jurnal_transaksi jt
        JOIN rekening r ON jt.id_rekening = r.id_rekening
        GROUP BY r.id_nasabah
    )
    SELECT AVG(saldo) AS rata_rata_saldo FROM saldo_nasabah;', '2025-11-11 10:51:35.063988+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (17, '-- Pertanyaan: "berapa nilai transaksi terbesar yang pernah terjadi?"', 'SELECT MAX(jumlah) AS transaksi_terbesar FROM jurnal_transaksi;', '2025-11-11 10:51:35.063988+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (18, '-- Pertanyaan: "tampilkan 5 nasabah dengan saldo paling sedikit"', 'SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo
     FROM jurnal_transaksi jt 
     JOIN rekening r ON jt.id_rekening = r.id_rekening 
     JOIN nasabah n ON r.id_nasabah = n.id_nasabah
     GROUP BY n.id_nasabah, n.nama_lengkap 
     ORDER BY total_saldo ASC 
     LIMIT 5;', '2025-11-11 10:51:35.063988+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (19, '-- Pertanyaan: "tampilkan nasabah dari jakarta yang punya tabungan"', 'SELECT n.id_nasabah, n.nama_lengkap, n.alamat
     FROM nasabah n
     JOIN rekening r ON n.id_nasabah = r.id_nasabah
     JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
     WHERE n.alamat ILIKE ''%jakarta%'' 
       AND mjr.nama_jenis = ''tabungan''
     GROUP BY n.id_nasabah, n.nama_lengkap, n.alamat;', '2025-11-11 10:51:47.253086+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (20, '-- Pertanyaan: "siapa 3 nasabah tabungan dengan saldo terbanyak?"', 'SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_tabungan
    FROM jurnal_transaksi jt JOIN rekening r ON jt.id_rekening = r.id_rekening JOIN nasabah n ON r.id_nasabah = n.id_nasabah JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
    WHERE mjr.nama_jenis = ''tabungan''
    GROUP BY n.id_nasabah, n.nama_lengkap ORDER BY total_saldo_tabungan DESC LIMIT 3;', '2025-11-11 10:51:47.253086+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (21, '-- Pertanyaan: "tampilkan nasabah dari jakarta yang punya tabungan"', 'SELECT n.id_nasabah, n.nama_lengkap, n.alamat
     FROM nasabah n
     JOIN rekening r ON n.id_nasabah = r.id_nasabah
     JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
     WHERE n.alamat ILIKE ''%jakarta%'' 
       AND mjr.nama_jenis = ''tabungan''
     GROUP BY n.id_nasabah, n.nama_lengkap, n.alamat;', '2025-11-11 10:51:54.948179+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (22, '-- Pertanyaan: "siapa 3 nasabah tabungan dengan saldo terbanyak?"', 'SELECT n.nama_lengkap, SUM(CASE WHEN jt.tipe_dk = ''KREDIT'' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_tabungan
    FROM jurnal_transaksi jt JOIN rekening r ON jt.id_rekening = r.id_rekening JOIN nasabah n ON r.id_nasabah = n.id_nasabah JOIN master_jenis_rekening mjr ON r.id_jenis_rekening = mjr.id_jenis_rekening
    WHERE mjr.nama_jenis = ''tabungan''
    GROUP BY n.id_nasabah, n.nama_lengkap ORDER BY total_saldo_tabungan DESC LIMIT 3;', '2025-11-11 10:51:54.948179+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (23, '-- Pertanyaan: "tampilkan nasabah yang tidak punya rekening"', 'SELECT n.id_nasabah, n.nama_lengkap
     FROM nasabah n
     LEFT JOIN rekening r ON n.id_nasabah = r.id_nasabah
     WHERE r.id_rekening IS NULL;', '2025-11-11 10:52:08.238879+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (24, '-- Pertanyaan: "tampilkan nasabah yang tidak pernah bertransaksi"', 'SELECT n.id_nasabah, n.nama_lengkap
     FROM nasabah n
     LEFT JOIN rekening r ON n.id_nasabah = r.id_nasabah
     LEFT JOIN jurnal_transaksi jt ON r.id_rekening = jt.id_rekening
     WHERE jt.id_transaksi IS NULL;', '2025-11-11 10:52:08.238879+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (25, '-- Pertanyaan: "tampilkan nasabah yang tidak punya rekening"', 'SELECT n.id_nasabah, n.nama_lengkap
     FROM nasabah n
     LEFT JOIN rekening r ON n.id_nasabah = r.id_nasabah
     WHERE r.id_rekening IS NULL;', '2025-11-12 10:20:18.052015+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (26, '-- Pertanyaan: "nasabah yang belum pernah transaksi"', 'SELECT n.id_nasabah, n.nama_lengkap
     FROM nasabah n
     LEFT JOIN rekening r ON n.id_nasabah = r.id_nasabah
     LEFT JOIN jurnal_transaksi jt ON r.id_rekening = jt.id_rekening
     WHERE jt.id_transaksi IS NULL;', '2025-11-12 10:20:18.052015+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (27, '-- Pertanyaan: "nasabah yang ga punya rekening"', 'SELECT n.id_nasabah, n.nama_lengkap FROM nasabah n LEFT JOIN rekening r ON n.id_nasah = r.id_nasabah WHERE r.id_rekening IS NULL;', '2025-11-12 10:36:45.031441+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (28, '-- Pertanyaan: "berapa total saldo dari semua nasabah?"', 'SELECT SUM(CASE WHEN jt.tipe_dk = ''''KREDIT'''' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_semua_nasabah FROM jurnal_transaksi jt;', '2025-11-12 14:34:07.842768+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (29, '-- Pertanyaan: "tolong hitung total uang semua nasabah"', 'SELECT SUM(CASE WHEN jt.tipe_dk = ''''KREDIT'''' THEN jt.jumlah ELSE -jt.jumlah END) AS total_saldo_semua_nasabah FROM jurnal_transaksi jt;', '2025-11-12 14:41:13.964661+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (30, '-- Pertanyaan: "agregat simpanan untuk seluruh nasabah"', 'SELECT SUM(CASE WHEN jt.tipe_dk = ''''KREDIT'''' THEN jt.jumlah ELSE -jt.jumlah END) AS agregat_simpanan FROM jurnal_transaksi jt;', '2025-11-12 15:43:31.223557+07');
INSERT INTO bpr_supra.rag_sql_examples VALUES (31, '-- Pertanyaan: "penabung atau nasabah huruf depan (hanya huruf depan antara a sampai z)"', 'SELECT id_nasabah, nama_lengkap FROM nasabah WHERE nama_lengkap ILIKE ''(huruf depan)%'';', '2025-11-13 10:03:56.821887+07');


--
-- TOC entry 4941 (class 0 OID 107458)
-- Dependencies: 227
-- Data for Name: rekening; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.rekening VALUES ('110000001', 'CIF00001', 1, 1, '2022-01-15');
INSERT INTO bpr_supra.rekening VALUES ('130000001', 'CIF00001', 3, 1, '2023-05-10');
INSERT INTO bpr_supra.rekening VALUES ('110000002', 'CIF00002', 1, 1, '2022-03-20');
INSERT INTO bpr_supra.rekening VALUES ('140000001', 'CIF00005', 4, 1, '2023-01-30');
INSERT INTO bpr_supra.rekening VALUES ('220000001', 'CIF10001', 2, 1, '2020-07-02');
INSERT INTO bpr_supra.rekening VALUES ('220000002', 'CIF10002', 2, 1, '2021-08-15');
INSERT INTO bpr_supra.rekening VALUES ('110000003', 'CIF00003', 1, 1, '2021-11-01');
INSERT INTO bpr_supra.rekening VALUES ('110000004', 'CIF00004', 1, 1, '2022-02-05');


--
-- TOC entry 4943 (class 0 OID 107480)
-- Dependencies: 229
-- Data for Name: transaksi; Type: TABLE DATA; Schema: bpr_supra; Owner: -
--

INSERT INTO bpr_supra.transaksi VALUES (2, '2025-10-02 14:30:00+07', 2, 'Penarikan tunai ATM Citra Lestari (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (3, '2025-10-03 11:05:00+07', 4, 'Transfer M-Banking dari 110000001 ke 110000002');
INSERT INTO bpr_supra.transaksi VALUES (4, '2025-10-31 23:59:00+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 110000001)');
INSERT INTO bpr_supra.transaksi VALUES (5, '2025-11-01 10:00:00+07', 3, 'Penerimaan dana dari PT. Rekan Bisnis (Rek: 220000001)');
INSERT INTO bpr_supra.transaksi VALUES (6, '2025-10-31 23:59:00+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (8, '2025-11-02 10:22:00+07', 2, 'Penarikan tunai Teller Budi Santoso (Rek: 110000001)');
INSERT INTO bpr_supra.transaksi VALUES (9, '2025-11-03 11:00:00+07', 1, 'Setoran tunai Eka Saputra (Rek: 140000001)');
INSERT INTO bpr_supra.transaksi VALUES (10, '2025-11-03 15:12:45+07', 2, 'Penarikan ATM (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (11, '2025-11-04 09:05:10+07', 4, 'Transfer dari PT. Maju Jaya (Rek: 220000001) ke Supplier');
INSERT INTO bpr_supra.transaksi VALUES (12, '2025-11-04 13:20:00+07', 3, 'Penerimaan Gaji Budi Santoso (Rek: 110000001)');
INSERT INTO bpr_supra.transaksi VALUES (17, '2025-10-31 23:59:01+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 220000001)');
INSERT INTO bpr_supra.transaksi VALUES (18, '2025-10-31 23:59:01+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 220000002)');
INSERT INTO bpr_supra.transaksi VALUES (19, '2025-10-31 23:59:01+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 140000001)');
INSERT INTO bpr_supra.transaksi VALUES (24, '2025-10-31 23:59:02+07', 5, 'Biaya admin bulanan Oktober 2023 (Rek: 130000001)');
INSERT INTO bpr_supra.transaksi VALUES (26, '2024-03-21 09:00:00+07', 1, 'Setoran Tunai Awal Citra Lestari (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (7, '2025-10-30 09:00:00+07', 1, 'Setoran tunai awal CV. Sinar Terang (Rek: 220000002)');
INSERT INTO bpr_supra.transaksi VALUES (25, '2025-11-03 17:45:10+07', 2, 'Penarikan ATM Budi Santoso (Rek: 110000001)');
INSERT INTO bpr_supra.transaksi VALUES (23, '2025-11-03 09:05:00+07', 3, 'Penerimaan dana dari marketplace (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (22, '2025-11-03 14:00:00+07', 2, 'Penarikan Tunai USD Eka Saputra (Rek: 140000001)');
INSERT INTO bpr_supra.transaksi VALUES (21, '2025-11-03 11:15:00+07', 1, 'Setoran Tunai USD Eka Saputra (Rek: 140000001)');
INSERT INTO bpr_supra.transaksi VALUES (20, '2025-11-03 10:30:00+07', 4, 'Transfer Giro dari PT. Maju Jaya ke CV. Sinar Terang');
INSERT INTO bpr_supra.transaksi VALUES (16, '2025-11-03 09:15:00+07', 2, 'Penarikan Cek/Giro No. CG12345 (Rek: 220000002)');
INSERT INTO bpr_supra.transaksi VALUES (15, '2025-11-03 14:35:00+07', 4, 'Pembayaran Gaji Karyawan (Rek: 220000001)');
INSERT INTO bpr_supra.transaksi VALUES (14, '2025-11-03 10:00:00+07', 3, 'Penerimaan Pembayaran dari Klien (Rek: 220000001)');
INSERT INTO bpr_supra.transaksi VALUES (13, '2025-11-03 16:45:00+07', 4, 'Pembayaran Listrik via M-Banking (Rek: 110000002)');
INSERT INTO bpr_supra.transaksi VALUES (1, '2025-08-01 09:15:00+07', 1, 'Setoran tunai Budi Santoso di Teller (Rek: 110000001)');


--
-- TOC entry 4964 (class 0 OID 0)
-- Dependencies: 230
-- Name: jurnal_transaksi_id_jurnal_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.jurnal_transaksi_id_jurnal_seq', 29, true);


--
-- TOC entry 4965 (class 0 OID 0)
-- Dependencies: 220
-- Name: master_jenis_rekening_id_jenis_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.master_jenis_rekening_id_jenis_seq', 4, true);


--
-- TOC entry 4966 (class 0 OID 0)
-- Dependencies: 222
-- Name: master_status_rekening_id_status_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.master_status_rekening_id_status_seq', 4, true);


--
-- TOC entry 4967 (class 0 OID 0)
-- Dependencies: 218
-- Name: master_tipe_nasabah_id_tipe_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.master_tipe_nasabah_id_tipe_seq', 3, true);


--
-- TOC entry 4968 (class 0 OID 0)
-- Dependencies: 224
-- Name: master_tipe_transaksi_id_tipe_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.master_tipe_transaksi_id_tipe_seq', 5, true);


--
-- TOC entry 4969 (class 0 OID 0)
-- Dependencies: 232
-- Name: rag_sql_examples_id_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.rag_sql_examples_id_seq', 31, true);


--
-- TOC entry 4970 (class 0 OID 0)
-- Dependencies: 228
-- Name: transaksi_id_transaksi_seq; Type: SEQUENCE SET; Schema: bpr_supra; Owner: -
--

SELECT pg_catalog.setval('bpr_supra.transaksi_id_transaksi_seq', 26, true);


--
-- TOC entry 4777 (class 2606 OID 107504)
-- Name: jurnal_transaksi jurnal_transaksi_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.jurnal_transaksi
    ADD CONSTRAINT jurnal_transaksi_pkey PRIMARY KEY (id_jurnal);


--
-- TOC entry 4755 (class 2606 OID 107427)
-- Name: master_jenis_rekening master_jenis_rekening_nama_jenis_key; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_jenis_rekening
    ADD CONSTRAINT master_jenis_rekening_nama_jenis_key UNIQUE (nama_jenis);


--
-- TOC entry 4757 (class 2606 OID 107425)
-- Name: master_jenis_rekening master_jenis_rekening_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_jenis_rekening
    ADD CONSTRAINT master_jenis_rekening_pkey PRIMARY KEY (id_jenis_rekening);


--
-- TOC entry 4759 (class 2606 OID 107436)
-- Name: master_status_rekening master_status_rekening_nama_status_key; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_status_rekening
    ADD CONSTRAINT master_status_rekening_nama_status_key UNIQUE (nama_status);


--
-- TOC entry 4761 (class 2606 OID 107434)
-- Name: master_status_rekening master_status_rekening_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_status_rekening
    ADD CONSTRAINT master_status_rekening_pkey PRIMARY KEY (id_status_rekening);


--
-- TOC entry 4751 (class 2606 OID 107418)
-- Name: master_tipe_nasabah master_tipe_nasabah_nama_tipe_key; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_nasabah
    ADD CONSTRAINT master_tipe_nasabah_nama_tipe_key UNIQUE (nama_tipe);


--
-- TOC entry 4753 (class 2606 OID 107416)
-- Name: master_tipe_nasabah master_tipe_nasabah_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_nasabah
    ADD CONSTRAINT master_tipe_nasabah_pkey PRIMARY KEY (id_tipe_nasabah);


--
-- TOC entry 4763 (class 2606 OID 107445)
-- Name: master_tipe_transaksi master_tipe_transaksi_kode_transaksi_key; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_transaksi
    ADD CONSTRAINT master_tipe_transaksi_kode_transaksi_key UNIQUE (kode_transaksi);


--
-- TOC entry 4765 (class 2606 OID 107443)
-- Name: master_tipe_transaksi master_tipe_transaksi_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.master_tipe_transaksi
    ADD CONSTRAINT master_tipe_transaksi_pkey PRIMARY KEY (id_tipe_transaksi);


--
-- TOC entry 4767 (class 2606 OID 107452)
-- Name: nasabah nasabah_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.nasabah
    ADD CONSTRAINT nasabah_pkey PRIMARY KEY (id_nasabah);


--
-- TOC entry 4779 (class 2606 OID 107562)
-- Name: rag_sql_examples rag_sql_examples_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rag_sql_examples
    ADD CONSTRAINT rag_sql_examples_pkey PRIMARY KEY (id);


--
-- TOC entry 4770 (class 2606 OID 107463)
-- Name: rekening rekening_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rekening
    ADD CONSTRAINT rekening_pkey PRIMARY KEY (id_rekening);


--
-- TOC entry 4773 (class 2606 OID 107486)
-- Name: transaksi transaksi_pkey; Type: CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.transaksi
    ADD CONSTRAINT transaksi_pkey PRIMARY KEY (id_transaksi);


--
-- TOC entry 4774 (class 1259 OID 107517)
-- Name: idx_jurnal_id_rekening; Type: INDEX; Schema: bpr_supra; Owner: -
--

CREATE INDEX idx_jurnal_id_rekening ON bpr_supra.jurnal_transaksi USING btree (id_rekening);


--
-- TOC entry 4775 (class 1259 OID 107516)
-- Name: idx_jurnal_id_transaksi; Type: INDEX; Schema: bpr_supra; Owner: -
--

CREATE INDEX idx_jurnal_id_transaksi ON bpr_supra.jurnal_transaksi USING btree (id_transaksi);


--
-- TOC entry 4768 (class 1259 OID 107515)
-- Name: idx_rekening_id_nasabah; Type: INDEX; Schema: bpr_supra; Owner: -
--

CREATE INDEX idx_rekening_id_nasabah ON bpr_supra.rekening USING btree (id_nasabah);


--
-- TOC entry 4771 (class 1259 OID 107518)
-- Name: idx_transaksi_waktu; Type: INDEX; Schema: bpr_supra; Owner: -
--

CREATE INDEX idx_transaksi_waktu ON bpr_supra.transaksi USING btree (waktu_transaksi);


--
-- TOC entry 4785 (class 2606 OID 107510)
-- Name: jurnal_transaksi jurnal_transaksi_id_rekening_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.jurnal_transaksi
    ADD CONSTRAINT jurnal_transaksi_id_rekening_fkey FOREIGN KEY (id_rekening) REFERENCES bpr_supra.rekening(id_rekening);


--
-- TOC entry 4786 (class 2606 OID 107505)
-- Name: jurnal_transaksi jurnal_transaksi_id_transaksi_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.jurnal_transaksi
    ADD CONSTRAINT jurnal_transaksi_id_transaksi_fkey FOREIGN KEY (id_transaksi) REFERENCES bpr_supra.transaksi(id_transaksi) ON DELETE RESTRICT;


--
-- TOC entry 4780 (class 2606 OID 107529)
-- Name: nasabah nasabah_id_tipe_nasabah_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.nasabah
    ADD CONSTRAINT nasabah_id_tipe_nasabah_fkey FOREIGN KEY (id_tipe_nasabah) REFERENCES bpr_supra.master_tipe_nasabah(id_tipe_nasabah);


--
-- TOC entry 4781 (class 2606 OID 107519)
-- Name: rekening rekening_id_jenis_rekening_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rekening
    ADD CONSTRAINT rekening_id_jenis_rekening_fkey FOREIGN KEY (id_jenis_rekening) REFERENCES bpr_supra.master_jenis_rekening(id_jenis_rekening);


--
-- TOC entry 4782 (class 2606 OID 107464)
-- Name: rekening rekening_id_nasabah_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rekening
    ADD CONSTRAINT rekening_id_nasabah_fkey FOREIGN KEY (id_nasabah) REFERENCES bpr_supra.nasabah(id_nasabah);


--
-- TOC entry 4783 (class 2606 OID 107524)
-- Name: rekening rekening_id_status_rekening_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.rekening
    ADD CONSTRAINT rekening_id_status_rekening_fkey FOREIGN KEY (id_status_rekening) REFERENCES bpr_supra.master_status_rekening(id_status_rekening);


--
-- TOC entry 4784 (class 2606 OID 107534)
-- Name: transaksi transaksi_id_tipe_transaksi_fkey; Type: FK CONSTRAINT; Schema: bpr_supra; Owner: -
--

ALTER TABLE ONLY bpr_supra.transaksi
    ADD CONSTRAINT transaksi_id_tipe_transaksi_fkey FOREIGN KEY (id_tipe_transaksi) REFERENCES bpr_supra.master_tipe_transaksi(id_tipe_transaksi);


-- Completed on 2025-11-13 10:38:05

--
-- PostgreSQL database dump complete
--


-- 001_eu_countries.up.sql
-- Reference table for all 27 EU member states

CREATE TABLE eu_countries (
    country_code TEXT PRIMARY KEY,             -- ISO 3166-1 alpha-2
    name TEXT NOT NULL,
    local_vat_name TEXT NOT NULL,              -- e.g., "Mehrwertsteuer", "Taxe sur la valeur ajoutée"
    local_vat_abbreviation TEXT NOT NULL,      -- e.g., "MwSt.", "TVA", "IVA"
    is_eu_member BOOLEAN NOT NULL DEFAULT true,
    currency TEXT NOT NULL DEFAULT 'EUR'
);

INSERT INTO eu_countries (country_code, name, local_vat_name, local_vat_abbreviation, currency) VALUES
    ('AT', 'Austria',        'Umsatzsteuer',                          'USt.',   'EUR'),
    ('BE', 'Belgium',        'Belasting over de toegevoegde waarde',  'BTW',    'EUR'),
    ('BG', 'Bulgaria',       'Данък добавена стойност',                'ДДС',    'BGN'),
    ('HR', 'Croatia',        'Porez na dodanu vrijednost',            'PDV',    'EUR'),
    ('CY', 'Cyprus',         'Φόρος Προστιθέμενης Αξίας',              'ΦΠΑ',    'EUR'),
    ('CZ', 'Czech Republic', 'Daň z přidané hodnoty',                'DPH',    'CZK'),
    ('DK', 'Denmark',        'Merværdiafgift',                       'moms',   'DKK'),
    ('EE', 'Estonia',        'Käibemaks',                             'km',     'EUR'),
    ('FI', 'Finland',        'Arvonlisävero',                        'ALV',    'EUR'),
    ('FR', 'France',         'Taxe sur la valeur ajoutée',           'TVA',    'EUR'),
    ('DE', 'Germany',        'Mehrwertsteuer',                       'MwSt.',  'EUR'),
    ('GR', 'Greece',         'Φόρος Προστιθέμενης Αξίας',              'ΦΠΑ',    'EUR'),
    ('HU', 'Hungary',        'Általános forgalmi adó',               'ÁFA',    'HUF'),
    ('IE', 'Ireland',        'Value-Added Tax',                      'VAT',    'EUR'),
    ('IT', 'Italy',          'Imposta sul valore aggiunto',          'IVA',    'EUR'),
    ('LV', 'Latvia',         'Pievienotās vērtības nodoklis',        'PVN',    'EUR'),
    ('LT', 'Lithuania',      'Pridėtinės vertės mokestis',           'PVM',    'EUR'),
    ('LU', 'Luxembourg',     'Taxe sur la valeur ajoutée',           'TVA',    'EUR'),
    ('MT', 'Malta',          'Taxxa fuq il-Valur Miżjud',           'TVA',    'EUR'),
    ('NL', 'Netherlands',    'Belasting over de toegevoegde waarde', 'BTW',    'EUR'),
    ('PL', 'Poland',         'Podatek od towarów i usług',           'PTU',    'PLN'),
    ('PT', 'Portugal',       'Imposto sobre o Valor Acrescentado',   'IVA',    'EUR'),
    ('RO', 'Romania',        'Taxa pe valoarea adăugată',            'TVA',    'RON'),
    ('SK', 'Slovakia',       'Daň z pridanej hodnoty',              'DPH',    'EUR'),
    ('SI', 'Slovenia',       'Davek na dodano vrednost',             'DDV',    'EUR'),
    ('ES', 'Spain',          'Impuesto sobre el Valor Añadido',      'IVA',    'EUR'),
    ('SE', 'Sweden',         'Mervärdesskatt',                       'moms',   'SEK');

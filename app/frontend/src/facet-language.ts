import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';

export function registerFacetLanguage() {
  monaco.languages.register({ id: 'facet' });

  monaco.languages.setMonarchTokensProvider('facet', {
    keywords: [
      'fn', 'var', 'const', 'return', 'rtn', 'for', 'yield', 'fold', 'from',
      'if', 'else', 'true', 'false', 'lib', 'self', 'type', 'assert', 'where',
    ],
    typeKeywords: [
      'Solid', 'Sketch', 'Length', 'Angle', 'Number',
      'Vec2', 'Vec3', 'Bool', 'Array', 'String', 'Library', 'Box',
    ],
    constants: [
      'PI', 'TAU', 'E',
    ],
    units: [
      'pm', 'picometer', 'picometers', 'nm', 'nanometer', 'nanometers',
      'um', 'micrometer', 'micrometers', 'mm', 'millimeter', 'millimeters',
      'cm', 'centimeter', 'centimeters', 'dm', 'decimeter', 'decimeters',
      'm', 'meter', 'meters', 'dam', 'decameter', 'decameters',
      'hm', 'hectometer', 'hectometers', 'km', 'kilometer', 'kilometers',
      'klick', 'klicks', 'Mm', 'megameter', 'megameters', 'Gm', 'gigameter', 'gigameters',
      'in', 'inch', 'inches', 'ft', 'foot', 'feet',
      'yd', 'yard', 'yards', 'mi', 'mile', 'miles', 'thou',
      'nmi', 'nautical_mile', 'nautical_miles', 'fathom', 'fathoms',
      'furlong', 'furlongs', 'chain', 'chains', 'rod', 'rods', 'league', 'leagues',
      'hand', 'hands', 'cubit', 'cubits', 'smoot', 'smoots', 'barleycorn', 'barleycorns',
      'parsec', 'parsecs', 'ly', 'light_year', 'light_years',
      'au', 'astronomical_unit', 'astronomical_units',
      'micron', 'microns', 'angstrom', 'angstroms',
      'kellicam', 'kellicams', 'hopper', 'hoppers',
      'potrzebie', 'potrzebies',
      'sheppey', 'sheppeys', 'beard_second', 'beard_seconds',
      'planck', 'plancks',
      'deg', 'degree', 'degrees', 'rad', 'radian', 'radians',
      'grad', 'grads', 'gon', 'gons', 'turn', 'turns', 'rev', 'revs',
      'arcmin', 'arcmins', 'arcsec', 'arcsecs', 'mrad', 'mrads', 'mil', 'mils',
      'compass_point', 'compass_points', 'sextant', 'sextants', 'quadrant', 'quadrants',
    ],
    operators: [
      '+=', '-=', '*=', '/=', '%=', '^=', '&=',
      '+', '-', '*', '/', '%', '^', '&',
      '==', '!=', '<', '>', '<=', '>=',
      '&&', '||', '=', ':',
    ],
    symbols: /[=><!~?:&|+\-*/^%]+/,

    tokenizer: {
      root: [
        // Comments: // and #
        [/\/\/.*$/, 'comment.line'],
        [/#.*$/, 'comment.line'],

        // Strings: "..." and raw `...` (backtick may span lines)
        [/"[^"]*"/, 'string.quoted.double'],
        [/`/, 'string.quoted.other', '@rawString'],

        // Numbers: ratios, floats, integers — enter afterNumber to detect trailing unit
        [/\d+\/\d+/, 'constant.numeric', '@afterNumber'],
        [/\d+\.\d+/, 'constant.numeric.float', '@afterNumber'],
        [/\d+/, 'constant.numeric', '@afterNumber'],

        // Unicode math constants
        [/[πτ]/, 'constant.language'],

        // Identifiers, keywords, types, constants (units only after numbers)
        [/[a-zA-Z_]\w*/, {
          cases: {
            '@keywords': 'keyword.control',
            '@typeKeywords': 'support.type',
            '@constants': 'constant.language',
            '@default': 'variable.other',
          },
        }],

        // Operators
        [/@symbols/, {
          cases: {
            '@operators': 'keyword.operator',
            '@default': '',
          },
        }],

        // Closing paren may be followed by a unit, e.g. (1/2) mm
        [/\)/, '@brackets', '@afterNumber'],

        // Other brackets
        [/[{}([\]]/, '@brackets'],

        // Delimiters
        [/[;,.]/, 'punctuation.delimiter'],
      ],

      // Raw backtick string — consume until closing backtick
      rawString: [
        [/`/, 'string.quoted.other', '@pop'],
        [/./, 'string.quoted.other'],
      ],

      // After a number literal, check if the next identifier is a unit
      afterNumber: [
        [/[ \t]+/, 'white'],
        [/[a-zA-Z_]\w*/, {
          cases: {
            '@units': { token: 'keyword.other.unit', next: '@pop' },
            '@default': { token: '@rematch', next: '@pop' },
          },
        }],
        [/$/, '', '@pop'],
        [/./, { token: '@rematch', next: '@pop' }],
      ],
    },
  });

  monaco.languages.setLanguageConfiguration('facet', {
    comments: {
      lineComment: '//',
    },
    brackets: [
      ['{', '}'],
      ['[', ']'],
      ['(', ')'],
    ],
    autoClosingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: '"', close: '"', notIn: ['string'] },
      { open: '`', close: '`', notIn: ['string'] },
    ],
    surroundingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: '"', close: '"' },
      { open: '`', close: '`' },
    ],
    indentationRules: {
      increaseIndentPattern: /\{[^}]*$/,
      decreaseIndentPattern: /^\s*\}/,
    },
    onEnterRules: [
      {
        // Enter inside { } — indent and place cursor between braces
        beforeText: /\{[^}]*$/,
        afterText: /^\s*\}/,
        action: { indentAction: monaco.languages.IndentAction.IndentOutdent },
      },
      {
        // Enter after { with nothing closing — just indent
        beforeText: /\{[^}]*$/,
        action: { indentAction: monaco.languages.IndentAction.Indent },
      },
    ],
  });
}

/// Display labels + units for the nutrient keys the backend uses in totals,
/// goals and adherence blocks. Keys match the JSON field names 1:1.
const Map<String, String> nutrientLabels = {
  'kcal': 'Energy',
  'protein_g': 'Protein',
  'carbs_g': 'Carbs',
  'fat_g': 'Fat',
  'fiber_g': 'Fiber',
  'sugar_g': 'Sugar',
  'salt_g': 'Salt',
  'iron_mg': 'Iron',
  'calcium_mg': 'Calcium',
  'vitamin_d_mcg': 'Vit D',
  'vitamin_b12_mcg': 'B12',
  'vitamin_c_mg': 'Vit C',
  'magnesium_mg': 'Magnesium',
  'potassium_mg': 'Potassium',
  'zinc_mg': 'Zinc',
};

const Map<String, String> nutrientUnits = {
  'kcal': 'kcal',
  'protein_g': 'g',
  'carbs_g': 'g',
  'fat_g': 'g',
  'fiber_g': 'g',
  'sugar_g': 'g',
  'salt_g': 'g',
  'iron_mg': 'mg',
  'calcium_mg': 'mg',
  'vitamin_d_mcg': 'mcg',
  'vitamin_b12_mcg': 'mcg',
  'vitamin_c_mg': 'mg',
  'magnesium_mg': 'mg',
  'potassium_mg': 'mg',
  'zinc_mg': 'mg',
};

String labelFor(String key) => nutrientLabels[key] ?? key;
String unitFor(String key) => nutrientUnits[key] ?? '';

/// The five headline nutrients promoted to rings on Today.
const List<String> ringNutrients = [
  'kcal',
  'protein_g',
  'fiber_g',
  'iron_mg',
  'vitamin_b12_mcg',
];

// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_database.dart';

// ignore_for_file: type=lint
class $ProductsCacheTable extends ProductsCache
    with TableInfo<$ProductsCacheTable, ProductsCacheData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $ProductsCacheTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
    'name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _brandMeta = const VerificationMeta('brand');
  @override
  late final GeneratedColumn<String> brand = GeneratedColumn<String>(
    'brand',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _sourceMeta = const VerificationMeta('source');
  @override
  late final GeneratedColumn<String> source = GeneratedColumn<String>(
    'source',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nutrimentsPer100gJsonMeta =
      const VerificationMeta('nutrimentsPer100gJson');
  @override
  late final GeneratedColumn<String> nutrimentsPer100gJson =
      GeneratedColumn<String>(
        'nutriments_per100g_json',
        aliasedName,
        false,
        type: DriftSqlType.string,
        requiredDuringInsert: true,
      );
  static const VerificationMeta _servingSizeGMeta = const VerificationMeta(
    'servingSizeG',
  );
  @override
  late final GeneratedColumn<double> servingSizeG = GeneratedColumn<double>(
    'serving_size_g',
    aliasedName,
    true,
    type: DriftSqlType.double,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _lastLoggedQuantityGMeta =
      const VerificationMeta('lastLoggedQuantityG');
  @override
  late final GeneratedColumn<double> lastLoggedQuantityG =
      GeneratedColumn<double>(
        'last_logged_quantity_g',
        aliasedName,
        true,
        type: DriftSqlType.double,
        requiredDuringInsert: false,
      );
  static const VerificationMeta _lastLoggedAtMeta = const VerificationMeta(
    'lastLoggedAt',
  );
  @override
  late final GeneratedColumn<DateTime> lastLoggedAt = GeneratedColumn<DateTime>(
    'last_logged_at',
    aliasedName,
    true,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _refreshedAtMeta = const VerificationMeta(
    'refreshedAt',
  );
  @override
  late final GeneratedColumn<DateTime> refreshedAt = GeneratedColumn<DateTime>(
    'refreshed_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    name,
    brand,
    source,
    nutrimentsPer100gJson,
    servingSizeG,
    lastLoggedQuantityG,
    lastLoggedAt,
    refreshedAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'products_cache';
  @override
  VerificationContext validateIntegrity(
    Insertable<ProductsCacheData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
        _nameMeta,
        name.isAcceptableOrUnknown(data['name']!, _nameMeta),
      );
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('brand')) {
      context.handle(
        _brandMeta,
        brand.isAcceptableOrUnknown(data['brand']!, _brandMeta),
      );
    }
    if (data.containsKey('source')) {
      context.handle(
        _sourceMeta,
        source.isAcceptableOrUnknown(data['source']!, _sourceMeta),
      );
    } else if (isInserting) {
      context.missing(_sourceMeta);
    }
    if (data.containsKey('nutriments_per100g_json')) {
      context.handle(
        _nutrimentsPer100gJsonMeta,
        nutrimentsPer100gJson.isAcceptableOrUnknown(
          data['nutriments_per100g_json']!,
          _nutrimentsPer100gJsonMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_nutrimentsPer100gJsonMeta);
    }
    if (data.containsKey('serving_size_g')) {
      context.handle(
        _servingSizeGMeta,
        servingSizeG.isAcceptableOrUnknown(
          data['serving_size_g']!,
          _servingSizeGMeta,
        ),
      );
    }
    if (data.containsKey('last_logged_quantity_g')) {
      context.handle(
        _lastLoggedQuantityGMeta,
        lastLoggedQuantityG.isAcceptableOrUnknown(
          data['last_logged_quantity_g']!,
          _lastLoggedQuantityGMeta,
        ),
      );
    }
    if (data.containsKey('last_logged_at')) {
      context.handle(
        _lastLoggedAtMeta,
        lastLoggedAt.isAcceptableOrUnknown(
          data['last_logged_at']!,
          _lastLoggedAtMeta,
        ),
      );
    }
    if (data.containsKey('refreshed_at')) {
      context.handle(
        _refreshedAtMeta,
        refreshedAt.isAcceptableOrUnknown(
          data['refreshed_at']!,
          _refreshedAtMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_refreshedAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  ProductsCacheData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return ProductsCacheData(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      name: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}name'],
      )!,
      brand: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}brand'],
      ),
      source: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}source'],
      )!,
      nutrimentsPer100gJson: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}nutriments_per100g_json'],
      )!,
      servingSizeG: attachedDatabase.typeMapping.read(
        DriftSqlType.double,
        data['${effectivePrefix}serving_size_g'],
      ),
      lastLoggedQuantityG: attachedDatabase.typeMapping.read(
        DriftSqlType.double,
        data['${effectivePrefix}last_logged_quantity_g'],
      ),
      lastLoggedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}last_logged_at'],
      ),
      refreshedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}refreshed_at'],
      )!,
    );
  }

  @override
  $ProductsCacheTable createAlias(String alias) {
    return $ProductsCacheTable(attachedDatabase, alias);
  }
}

class ProductsCacheData extends DataClass
    implements Insertable<ProductsCacheData> {
  final String id;
  final String name;
  final String? brand;
  final String source;
  final String nutrimentsPer100gJson;
  final double? servingSizeG;
  final double? lastLoggedQuantityG;
  final DateTime? lastLoggedAt;
  final DateTime refreshedAt;
  const ProductsCacheData({
    required this.id,
    required this.name,
    this.brand,
    required this.source,
    required this.nutrimentsPer100gJson,
    this.servingSizeG,
    this.lastLoggedQuantityG,
    this.lastLoggedAt,
    required this.refreshedAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['name'] = Variable<String>(name);
    if (!nullToAbsent || brand != null) {
      map['brand'] = Variable<String>(brand);
    }
    map['source'] = Variable<String>(source);
    map['nutriments_per100g_json'] = Variable<String>(nutrimentsPer100gJson);
    if (!nullToAbsent || servingSizeG != null) {
      map['serving_size_g'] = Variable<double>(servingSizeG);
    }
    if (!nullToAbsent || lastLoggedQuantityG != null) {
      map['last_logged_quantity_g'] = Variable<double>(lastLoggedQuantityG);
    }
    if (!nullToAbsent || lastLoggedAt != null) {
      map['last_logged_at'] = Variable<DateTime>(lastLoggedAt);
    }
    map['refreshed_at'] = Variable<DateTime>(refreshedAt);
    return map;
  }

  ProductsCacheCompanion toCompanion(bool nullToAbsent) {
    return ProductsCacheCompanion(
      id: Value(id),
      name: Value(name),
      brand: brand == null && nullToAbsent
          ? const Value.absent()
          : Value(brand),
      source: Value(source),
      nutrimentsPer100gJson: Value(nutrimentsPer100gJson),
      servingSizeG: servingSizeG == null && nullToAbsent
          ? const Value.absent()
          : Value(servingSizeG),
      lastLoggedQuantityG: lastLoggedQuantityG == null && nullToAbsent
          ? const Value.absent()
          : Value(lastLoggedQuantityG),
      lastLoggedAt: lastLoggedAt == null && nullToAbsent
          ? const Value.absent()
          : Value(lastLoggedAt),
      refreshedAt: Value(refreshedAt),
    );
  }

  factory ProductsCacheData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return ProductsCacheData(
      id: serializer.fromJson<String>(json['id']),
      name: serializer.fromJson<String>(json['name']),
      brand: serializer.fromJson<String?>(json['brand']),
      source: serializer.fromJson<String>(json['source']),
      nutrimentsPer100gJson: serializer.fromJson<String>(
        json['nutrimentsPer100gJson'],
      ),
      servingSizeG: serializer.fromJson<double?>(json['servingSizeG']),
      lastLoggedQuantityG: serializer.fromJson<double?>(
        json['lastLoggedQuantityG'],
      ),
      lastLoggedAt: serializer.fromJson<DateTime?>(json['lastLoggedAt']),
      refreshedAt: serializer.fromJson<DateTime>(json['refreshedAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'name': serializer.toJson<String>(name),
      'brand': serializer.toJson<String?>(brand),
      'source': serializer.toJson<String>(source),
      'nutrimentsPer100gJson': serializer.toJson<String>(nutrimentsPer100gJson),
      'servingSizeG': serializer.toJson<double?>(servingSizeG),
      'lastLoggedQuantityG': serializer.toJson<double?>(lastLoggedQuantityG),
      'lastLoggedAt': serializer.toJson<DateTime?>(lastLoggedAt),
      'refreshedAt': serializer.toJson<DateTime>(refreshedAt),
    };
  }

  ProductsCacheData copyWith({
    String? id,
    String? name,
    Value<String?> brand = const Value.absent(),
    String? source,
    String? nutrimentsPer100gJson,
    Value<double?> servingSizeG = const Value.absent(),
    Value<double?> lastLoggedQuantityG = const Value.absent(),
    Value<DateTime?> lastLoggedAt = const Value.absent(),
    DateTime? refreshedAt,
  }) => ProductsCacheData(
    id: id ?? this.id,
    name: name ?? this.name,
    brand: brand.present ? brand.value : this.brand,
    source: source ?? this.source,
    nutrimentsPer100gJson: nutrimentsPer100gJson ?? this.nutrimentsPer100gJson,
    servingSizeG: servingSizeG.present ? servingSizeG.value : this.servingSizeG,
    lastLoggedQuantityG: lastLoggedQuantityG.present
        ? lastLoggedQuantityG.value
        : this.lastLoggedQuantityG,
    lastLoggedAt: lastLoggedAt.present ? lastLoggedAt.value : this.lastLoggedAt,
    refreshedAt: refreshedAt ?? this.refreshedAt,
  );
  ProductsCacheData copyWithCompanion(ProductsCacheCompanion data) {
    return ProductsCacheData(
      id: data.id.present ? data.id.value : this.id,
      name: data.name.present ? data.name.value : this.name,
      brand: data.brand.present ? data.brand.value : this.brand,
      source: data.source.present ? data.source.value : this.source,
      nutrimentsPer100gJson: data.nutrimentsPer100gJson.present
          ? data.nutrimentsPer100gJson.value
          : this.nutrimentsPer100gJson,
      servingSizeG: data.servingSizeG.present
          ? data.servingSizeG.value
          : this.servingSizeG,
      lastLoggedQuantityG: data.lastLoggedQuantityG.present
          ? data.lastLoggedQuantityG.value
          : this.lastLoggedQuantityG,
      lastLoggedAt: data.lastLoggedAt.present
          ? data.lastLoggedAt.value
          : this.lastLoggedAt,
      refreshedAt: data.refreshedAt.present
          ? data.refreshedAt.value
          : this.refreshedAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('ProductsCacheData(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('brand: $brand, ')
          ..write('source: $source, ')
          ..write('nutrimentsPer100gJson: $nutrimentsPer100gJson, ')
          ..write('servingSizeG: $servingSizeG, ')
          ..write('lastLoggedQuantityG: $lastLoggedQuantityG, ')
          ..write('lastLoggedAt: $lastLoggedAt, ')
          ..write('refreshedAt: $refreshedAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    name,
    brand,
    source,
    nutrimentsPer100gJson,
    servingSizeG,
    lastLoggedQuantityG,
    lastLoggedAt,
    refreshedAt,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is ProductsCacheData &&
          other.id == this.id &&
          other.name == this.name &&
          other.brand == this.brand &&
          other.source == this.source &&
          other.nutrimentsPer100gJson == this.nutrimentsPer100gJson &&
          other.servingSizeG == this.servingSizeG &&
          other.lastLoggedQuantityG == this.lastLoggedQuantityG &&
          other.lastLoggedAt == this.lastLoggedAt &&
          other.refreshedAt == this.refreshedAt);
}

class ProductsCacheCompanion extends UpdateCompanion<ProductsCacheData> {
  final Value<String> id;
  final Value<String> name;
  final Value<String?> brand;
  final Value<String> source;
  final Value<String> nutrimentsPer100gJson;
  final Value<double?> servingSizeG;
  final Value<double?> lastLoggedQuantityG;
  final Value<DateTime?> lastLoggedAt;
  final Value<DateTime> refreshedAt;
  final Value<int> rowid;
  const ProductsCacheCompanion({
    this.id = const Value.absent(),
    this.name = const Value.absent(),
    this.brand = const Value.absent(),
    this.source = const Value.absent(),
    this.nutrimentsPer100gJson = const Value.absent(),
    this.servingSizeG = const Value.absent(),
    this.lastLoggedQuantityG = const Value.absent(),
    this.lastLoggedAt = const Value.absent(),
    this.refreshedAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  ProductsCacheCompanion.insert({
    required String id,
    required String name,
    this.brand = const Value.absent(),
    required String source,
    required String nutrimentsPer100gJson,
    this.servingSizeG = const Value.absent(),
    this.lastLoggedQuantityG = const Value.absent(),
    this.lastLoggedAt = const Value.absent(),
    required DateTime refreshedAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       name = Value(name),
       source = Value(source),
       nutrimentsPer100gJson = Value(nutrimentsPer100gJson),
       refreshedAt = Value(refreshedAt);
  static Insertable<ProductsCacheData> custom({
    Expression<String>? id,
    Expression<String>? name,
    Expression<String>? brand,
    Expression<String>? source,
    Expression<String>? nutrimentsPer100gJson,
    Expression<double>? servingSizeG,
    Expression<double>? lastLoggedQuantityG,
    Expression<DateTime>? lastLoggedAt,
    Expression<DateTime>? refreshedAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (name != null) 'name': name,
      if (brand != null) 'brand': brand,
      if (source != null) 'source': source,
      if (nutrimentsPer100gJson != null)
        'nutriments_per100g_json': nutrimentsPer100gJson,
      if (servingSizeG != null) 'serving_size_g': servingSizeG,
      if (lastLoggedQuantityG != null)
        'last_logged_quantity_g': lastLoggedQuantityG,
      if (lastLoggedAt != null) 'last_logged_at': lastLoggedAt,
      if (refreshedAt != null) 'refreshed_at': refreshedAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  ProductsCacheCompanion copyWith({
    Value<String>? id,
    Value<String>? name,
    Value<String?>? brand,
    Value<String>? source,
    Value<String>? nutrimentsPer100gJson,
    Value<double?>? servingSizeG,
    Value<double?>? lastLoggedQuantityG,
    Value<DateTime?>? lastLoggedAt,
    Value<DateTime>? refreshedAt,
    Value<int>? rowid,
  }) {
    return ProductsCacheCompanion(
      id: id ?? this.id,
      name: name ?? this.name,
      brand: brand ?? this.brand,
      source: source ?? this.source,
      nutrimentsPer100gJson:
          nutrimentsPer100gJson ?? this.nutrimentsPer100gJson,
      servingSizeG: servingSizeG ?? this.servingSizeG,
      lastLoggedQuantityG: lastLoggedQuantityG ?? this.lastLoggedQuantityG,
      lastLoggedAt: lastLoggedAt ?? this.lastLoggedAt,
      refreshedAt: refreshedAt ?? this.refreshedAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (brand.present) {
      map['brand'] = Variable<String>(brand.value);
    }
    if (source.present) {
      map['source'] = Variable<String>(source.value);
    }
    if (nutrimentsPer100gJson.present) {
      map['nutriments_per100g_json'] = Variable<String>(
        nutrimentsPer100gJson.value,
      );
    }
    if (servingSizeG.present) {
      map['serving_size_g'] = Variable<double>(servingSizeG.value);
    }
    if (lastLoggedQuantityG.present) {
      map['last_logged_quantity_g'] = Variable<double>(
        lastLoggedQuantityG.value,
      );
    }
    if (lastLoggedAt.present) {
      map['last_logged_at'] = Variable<DateTime>(lastLoggedAt.value);
    }
    if (refreshedAt.present) {
      map['refreshed_at'] = Variable<DateTime>(refreshedAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('ProductsCacheCompanion(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('brand: $brand, ')
          ..write('source: $source, ')
          ..write('nutrimentsPer100gJson: $nutrimentsPer100gJson, ')
          ..write('servingSizeG: $servingSizeG, ')
          ..write('lastLoggedQuantityG: $lastLoggedQuantityG, ')
          ..write('lastLoggedAt: $lastLoggedAt, ')
          ..write('refreshedAt: $refreshedAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $RecentSummaryTable extends RecentSummary
    with TableInfo<$RecentSummaryTable, RecentSummaryData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $RecentSummaryTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _dateMeta = const VerificationMeta('date');
  @override
  late final GeneratedColumn<String> date = GeneratedColumn<String>(
    'date',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _tzMeta = const VerificationMeta('tz');
  @override
  late final GeneratedColumn<String> tz = GeneratedColumn<String>(
    'tz',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _totalsJsonMeta = const VerificationMeta(
    'totalsJson',
  );
  @override
  late final GeneratedColumn<String> totalsJson = GeneratedColumn<String>(
    'totals_json',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _entriesJsonMeta = const VerificationMeta(
    'entriesJson',
  );
  @override
  late final GeneratedColumn<String> entriesJson = GeneratedColumn<String>(
    'entries_json',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _refreshedAtMeta = const VerificationMeta(
    'refreshedAt',
  );
  @override
  late final GeneratedColumn<DateTime> refreshedAt = GeneratedColumn<DateTime>(
    'refreshed_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    date,
    tz,
    totalsJson,
    entriesJson,
    refreshedAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'recent_summary';
  @override
  VerificationContext validateIntegrity(
    Insertable<RecentSummaryData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('date')) {
      context.handle(
        _dateMeta,
        date.isAcceptableOrUnknown(data['date']!, _dateMeta),
      );
    } else if (isInserting) {
      context.missing(_dateMeta);
    }
    if (data.containsKey('tz')) {
      context.handle(_tzMeta, tz.isAcceptableOrUnknown(data['tz']!, _tzMeta));
    } else if (isInserting) {
      context.missing(_tzMeta);
    }
    if (data.containsKey('totals_json')) {
      context.handle(
        _totalsJsonMeta,
        totalsJson.isAcceptableOrUnknown(data['totals_json']!, _totalsJsonMeta),
      );
    } else if (isInserting) {
      context.missing(_totalsJsonMeta);
    }
    if (data.containsKey('entries_json')) {
      context.handle(
        _entriesJsonMeta,
        entriesJson.isAcceptableOrUnknown(
          data['entries_json']!,
          _entriesJsonMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_entriesJsonMeta);
    }
    if (data.containsKey('refreshed_at')) {
      context.handle(
        _refreshedAtMeta,
        refreshedAt.isAcceptableOrUnknown(
          data['refreshed_at']!,
          _refreshedAtMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_refreshedAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {date, tz};
  @override
  RecentSummaryData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return RecentSummaryData(
      date: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}date'],
      )!,
      tz: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}tz'],
      )!,
      totalsJson: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}totals_json'],
      )!,
      entriesJson: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}entries_json'],
      )!,
      refreshedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}refreshed_at'],
      )!,
    );
  }

  @override
  $RecentSummaryTable createAlias(String alias) {
    return $RecentSummaryTable(attachedDatabase, alias);
  }
}

class RecentSummaryData extends DataClass
    implements Insertable<RecentSummaryData> {
  final String date;
  final String tz;
  final String totalsJson;
  final String entriesJson;
  final DateTime refreshedAt;
  const RecentSummaryData({
    required this.date,
    required this.tz,
    required this.totalsJson,
    required this.entriesJson,
    required this.refreshedAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['date'] = Variable<String>(date);
    map['tz'] = Variable<String>(tz);
    map['totals_json'] = Variable<String>(totalsJson);
    map['entries_json'] = Variable<String>(entriesJson);
    map['refreshed_at'] = Variable<DateTime>(refreshedAt);
    return map;
  }

  RecentSummaryCompanion toCompanion(bool nullToAbsent) {
    return RecentSummaryCompanion(
      date: Value(date),
      tz: Value(tz),
      totalsJson: Value(totalsJson),
      entriesJson: Value(entriesJson),
      refreshedAt: Value(refreshedAt),
    );
  }

  factory RecentSummaryData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return RecentSummaryData(
      date: serializer.fromJson<String>(json['date']),
      tz: serializer.fromJson<String>(json['tz']),
      totalsJson: serializer.fromJson<String>(json['totalsJson']),
      entriesJson: serializer.fromJson<String>(json['entriesJson']),
      refreshedAt: serializer.fromJson<DateTime>(json['refreshedAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'date': serializer.toJson<String>(date),
      'tz': serializer.toJson<String>(tz),
      'totalsJson': serializer.toJson<String>(totalsJson),
      'entriesJson': serializer.toJson<String>(entriesJson),
      'refreshedAt': serializer.toJson<DateTime>(refreshedAt),
    };
  }

  RecentSummaryData copyWith({
    String? date,
    String? tz,
    String? totalsJson,
    String? entriesJson,
    DateTime? refreshedAt,
  }) => RecentSummaryData(
    date: date ?? this.date,
    tz: tz ?? this.tz,
    totalsJson: totalsJson ?? this.totalsJson,
    entriesJson: entriesJson ?? this.entriesJson,
    refreshedAt: refreshedAt ?? this.refreshedAt,
  );
  RecentSummaryData copyWithCompanion(RecentSummaryCompanion data) {
    return RecentSummaryData(
      date: data.date.present ? data.date.value : this.date,
      tz: data.tz.present ? data.tz.value : this.tz,
      totalsJson: data.totalsJson.present
          ? data.totalsJson.value
          : this.totalsJson,
      entriesJson: data.entriesJson.present
          ? data.entriesJson.value
          : this.entriesJson,
      refreshedAt: data.refreshedAt.present
          ? data.refreshedAt.value
          : this.refreshedAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('RecentSummaryData(')
          ..write('date: $date, ')
          ..write('tz: $tz, ')
          ..write('totalsJson: $totalsJson, ')
          ..write('entriesJson: $entriesJson, ')
          ..write('refreshedAt: $refreshedAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode =>
      Object.hash(date, tz, totalsJson, entriesJson, refreshedAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is RecentSummaryData &&
          other.date == this.date &&
          other.tz == this.tz &&
          other.totalsJson == this.totalsJson &&
          other.entriesJson == this.entriesJson &&
          other.refreshedAt == this.refreshedAt);
}

class RecentSummaryCompanion extends UpdateCompanion<RecentSummaryData> {
  final Value<String> date;
  final Value<String> tz;
  final Value<String> totalsJson;
  final Value<String> entriesJson;
  final Value<DateTime> refreshedAt;
  final Value<int> rowid;
  const RecentSummaryCompanion({
    this.date = const Value.absent(),
    this.tz = const Value.absent(),
    this.totalsJson = const Value.absent(),
    this.entriesJson = const Value.absent(),
    this.refreshedAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  RecentSummaryCompanion.insert({
    required String date,
    required String tz,
    required String totalsJson,
    required String entriesJson,
    required DateTime refreshedAt,
    this.rowid = const Value.absent(),
  }) : date = Value(date),
       tz = Value(tz),
       totalsJson = Value(totalsJson),
       entriesJson = Value(entriesJson),
       refreshedAt = Value(refreshedAt);
  static Insertable<RecentSummaryData> custom({
    Expression<String>? date,
    Expression<String>? tz,
    Expression<String>? totalsJson,
    Expression<String>? entriesJson,
    Expression<DateTime>? refreshedAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (date != null) 'date': date,
      if (tz != null) 'tz': tz,
      if (totalsJson != null) 'totals_json': totalsJson,
      if (entriesJson != null) 'entries_json': entriesJson,
      if (refreshedAt != null) 'refreshed_at': refreshedAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  RecentSummaryCompanion copyWith({
    Value<String>? date,
    Value<String>? tz,
    Value<String>? totalsJson,
    Value<String>? entriesJson,
    Value<DateTime>? refreshedAt,
    Value<int>? rowid,
  }) {
    return RecentSummaryCompanion(
      date: date ?? this.date,
      tz: tz ?? this.tz,
      totalsJson: totalsJson ?? this.totalsJson,
      entriesJson: entriesJson ?? this.entriesJson,
      refreshedAt: refreshedAt ?? this.refreshedAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (date.present) {
      map['date'] = Variable<String>(date.value);
    }
    if (tz.present) {
      map['tz'] = Variable<String>(tz.value);
    }
    if (totalsJson.present) {
      map['totals_json'] = Variable<String>(totalsJson.value);
    }
    if (entriesJson.present) {
      map['entries_json'] = Variable<String>(entriesJson.value);
    }
    if (refreshedAt.present) {
      map['refreshed_at'] = Variable<DateTime>(refreshedAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('RecentSummaryCompanion(')
          ..write('date: $date, ')
          ..write('tz: $tz, ')
          ..write('totalsJson: $totalsJson, ')
          ..write('entriesJson: $entriesJson, ')
          ..write('refreshedAt: $refreshedAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $PendingWritesTable extends PendingWrites
    with TableInfo<$PendingWritesTable, PendingWrite> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $PendingWritesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _methodMeta = const VerificationMeta('method');
  @override
  late final GeneratedColumn<String> method = GeneratedColumn<String>(
    'method',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _pathMeta = const VerificationMeta('path');
  @override
  late final GeneratedColumn<String> path = GeneratedColumn<String>(
    'path',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _bodyMeta = const VerificationMeta('body');
  @override
  late final GeneratedColumn<Uint8List> body = GeneratedColumn<Uint8List>(
    'body',
    aliasedName,
    false,
    type: DriftSqlType.blob,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _idemKeyMeta = const VerificationMeta(
    'idemKey',
  );
  @override
  late final GeneratedColumn<String> idemKey = GeneratedColumn<String>(
    'idem_key',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _createdAtMeta = const VerificationMeta(
    'createdAt',
  );
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
    'created_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _statusMeta = const VerificationMeta('status');
  @override
  late final GeneratedColumn<String> status = GeneratedColumn<String>(
    'status',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant('pending'),
  );
  static const VerificationMeta _lastAttemptAtMeta = const VerificationMeta(
    'lastAttemptAt',
  );
  @override
  late final GeneratedColumn<DateTime> lastAttemptAt =
      GeneratedColumn<DateTime>(
        'last_attempt_at',
        aliasedName,
        true,
        type: DriftSqlType.dateTime,
        requiredDuringInsert: false,
      );
  static const VerificationMeta _attemptCountMeta = const VerificationMeta(
    'attemptCount',
  );
  @override
  late final GeneratedColumn<int> attemptCount = GeneratedColumn<int>(
    'attempt_count',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultValue: const Constant(0),
  );
  static const VerificationMeta _lastErrorMeta = const VerificationMeta(
    'lastError',
  );
  @override
  late final GeneratedColumn<String> lastError = GeneratedColumn<String>(
    'last_error',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    method,
    path,
    body,
    idemKey,
    createdAt,
    status,
    lastAttemptAt,
    attemptCount,
    lastError,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'pending_writes';
  @override
  VerificationContext validateIntegrity(
    Insertable<PendingWrite> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('method')) {
      context.handle(
        _methodMeta,
        method.isAcceptableOrUnknown(data['method']!, _methodMeta),
      );
    } else if (isInserting) {
      context.missing(_methodMeta);
    }
    if (data.containsKey('path')) {
      context.handle(
        _pathMeta,
        path.isAcceptableOrUnknown(data['path']!, _pathMeta),
      );
    } else if (isInserting) {
      context.missing(_pathMeta);
    }
    if (data.containsKey('body')) {
      context.handle(
        _bodyMeta,
        body.isAcceptableOrUnknown(data['body']!, _bodyMeta),
      );
    } else if (isInserting) {
      context.missing(_bodyMeta);
    }
    if (data.containsKey('idem_key')) {
      context.handle(
        _idemKeyMeta,
        idemKey.isAcceptableOrUnknown(data['idem_key']!, _idemKeyMeta),
      );
    } else if (isInserting) {
      context.missing(_idemKeyMeta);
    }
    if (data.containsKey('created_at')) {
      context.handle(
        _createdAtMeta,
        createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta),
      );
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    if (data.containsKey('status')) {
      context.handle(
        _statusMeta,
        status.isAcceptableOrUnknown(data['status']!, _statusMeta),
      );
    }
    if (data.containsKey('last_attempt_at')) {
      context.handle(
        _lastAttemptAtMeta,
        lastAttemptAt.isAcceptableOrUnknown(
          data['last_attempt_at']!,
          _lastAttemptAtMeta,
        ),
      );
    }
    if (data.containsKey('attempt_count')) {
      context.handle(
        _attemptCountMeta,
        attemptCount.isAcceptableOrUnknown(
          data['attempt_count']!,
          _attemptCountMeta,
        ),
      );
    }
    if (data.containsKey('last_error')) {
      context.handle(
        _lastErrorMeta,
        lastError.isAcceptableOrUnknown(data['last_error']!, _lastErrorMeta),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  PendingWrite map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return PendingWrite(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      method: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}method'],
      )!,
      path: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}path'],
      )!,
      body: attachedDatabase.typeMapping.read(
        DriftSqlType.blob,
        data['${effectivePrefix}body'],
      )!,
      idemKey: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}idem_key'],
      )!,
      createdAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}created_at'],
      )!,
      status: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}status'],
      )!,
      lastAttemptAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}last_attempt_at'],
      ),
      attemptCount: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}attempt_count'],
      )!,
      lastError: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}last_error'],
      ),
    );
  }

  @override
  $PendingWritesTable createAlias(String alias) {
    return $PendingWritesTable(attachedDatabase, alias);
  }
}

class PendingWrite extends DataClass implements Insertable<PendingWrite> {
  final String id;
  final String method;
  final String path;
  final Uint8List body;
  final String idemKey;
  final DateTime createdAt;
  final String status;
  final DateTime? lastAttemptAt;
  final int attemptCount;
  final String? lastError;
  const PendingWrite({
    required this.id,
    required this.method,
    required this.path,
    required this.body,
    required this.idemKey,
    required this.createdAt,
    required this.status,
    this.lastAttemptAt,
    required this.attemptCount,
    this.lastError,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['method'] = Variable<String>(method);
    map['path'] = Variable<String>(path);
    map['body'] = Variable<Uint8List>(body);
    map['idem_key'] = Variable<String>(idemKey);
    map['created_at'] = Variable<DateTime>(createdAt);
    map['status'] = Variable<String>(status);
    if (!nullToAbsent || lastAttemptAt != null) {
      map['last_attempt_at'] = Variable<DateTime>(lastAttemptAt);
    }
    map['attempt_count'] = Variable<int>(attemptCount);
    if (!nullToAbsent || lastError != null) {
      map['last_error'] = Variable<String>(lastError);
    }
    return map;
  }

  PendingWritesCompanion toCompanion(bool nullToAbsent) {
    return PendingWritesCompanion(
      id: Value(id),
      method: Value(method),
      path: Value(path),
      body: Value(body),
      idemKey: Value(idemKey),
      createdAt: Value(createdAt),
      status: Value(status),
      lastAttemptAt: lastAttemptAt == null && nullToAbsent
          ? const Value.absent()
          : Value(lastAttemptAt),
      attemptCount: Value(attemptCount),
      lastError: lastError == null && nullToAbsent
          ? const Value.absent()
          : Value(lastError),
    );
  }

  factory PendingWrite.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return PendingWrite(
      id: serializer.fromJson<String>(json['id']),
      method: serializer.fromJson<String>(json['method']),
      path: serializer.fromJson<String>(json['path']),
      body: serializer.fromJson<Uint8List>(json['body']),
      idemKey: serializer.fromJson<String>(json['idemKey']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
      status: serializer.fromJson<String>(json['status']),
      lastAttemptAt: serializer.fromJson<DateTime?>(json['lastAttemptAt']),
      attemptCount: serializer.fromJson<int>(json['attemptCount']),
      lastError: serializer.fromJson<String?>(json['lastError']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'method': serializer.toJson<String>(method),
      'path': serializer.toJson<String>(path),
      'body': serializer.toJson<Uint8List>(body),
      'idemKey': serializer.toJson<String>(idemKey),
      'createdAt': serializer.toJson<DateTime>(createdAt),
      'status': serializer.toJson<String>(status),
      'lastAttemptAt': serializer.toJson<DateTime?>(lastAttemptAt),
      'attemptCount': serializer.toJson<int>(attemptCount),
      'lastError': serializer.toJson<String?>(lastError),
    };
  }

  PendingWrite copyWith({
    String? id,
    String? method,
    String? path,
    Uint8List? body,
    String? idemKey,
    DateTime? createdAt,
    String? status,
    Value<DateTime?> lastAttemptAt = const Value.absent(),
    int? attemptCount,
    Value<String?> lastError = const Value.absent(),
  }) => PendingWrite(
    id: id ?? this.id,
    method: method ?? this.method,
    path: path ?? this.path,
    body: body ?? this.body,
    idemKey: idemKey ?? this.idemKey,
    createdAt: createdAt ?? this.createdAt,
    status: status ?? this.status,
    lastAttemptAt: lastAttemptAt.present
        ? lastAttemptAt.value
        : this.lastAttemptAt,
    attemptCount: attemptCount ?? this.attemptCount,
    lastError: lastError.present ? lastError.value : this.lastError,
  );
  PendingWrite copyWithCompanion(PendingWritesCompanion data) {
    return PendingWrite(
      id: data.id.present ? data.id.value : this.id,
      method: data.method.present ? data.method.value : this.method,
      path: data.path.present ? data.path.value : this.path,
      body: data.body.present ? data.body.value : this.body,
      idemKey: data.idemKey.present ? data.idemKey.value : this.idemKey,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
      status: data.status.present ? data.status.value : this.status,
      lastAttemptAt: data.lastAttemptAt.present
          ? data.lastAttemptAt.value
          : this.lastAttemptAt,
      attemptCount: data.attemptCount.present
          ? data.attemptCount.value
          : this.attemptCount,
      lastError: data.lastError.present ? data.lastError.value : this.lastError,
    );
  }

  @override
  String toString() {
    return (StringBuffer('PendingWrite(')
          ..write('id: $id, ')
          ..write('method: $method, ')
          ..write('path: $path, ')
          ..write('body: $body, ')
          ..write('idemKey: $idemKey, ')
          ..write('createdAt: $createdAt, ')
          ..write('status: $status, ')
          ..write('lastAttemptAt: $lastAttemptAt, ')
          ..write('attemptCount: $attemptCount, ')
          ..write('lastError: $lastError')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    method,
    path,
    $driftBlobEquality.hash(body),
    idemKey,
    createdAt,
    status,
    lastAttemptAt,
    attemptCount,
    lastError,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is PendingWrite &&
          other.id == this.id &&
          other.method == this.method &&
          other.path == this.path &&
          $driftBlobEquality.equals(other.body, this.body) &&
          other.idemKey == this.idemKey &&
          other.createdAt == this.createdAt &&
          other.status == this.status &&
          other.lastAttemptAt == this.lastAttemptAt &&
          other.attemptCount == this.attemptCount &&
          other.lastError == this.lastError);
}

class PendingWritesCompanion extends UpdateCompanion<PendingWrite> {
  final Value<String> id;
  final Value<String> method;
  final Value<String> path;
  final Value<Uint8List> body;
  final Value<String> idemKey;
  final Value<DateTime> createdAt;
  final Value<String> status;
  final Value<DateTime?> lastAttemptAt;
  final Value<int> attemptCount;
  final Value<String?> lastError;
  final Value<int> rowid;
  const PendingWritesCompanion({
    this.id = const Value.absent(),
    this.method = const Value.absent(),
    this.path = const Value.absent(),
    this.body = const Value.absent(),
    this.idemKey = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.status = const Value.absent(),
    this.lastAttemptAt = const Value.absent(),
    this.attemptCount = const Value.absent(),
    this.lastError = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  PendingWritesCompanion.insert({
    required String id,
    required String method,
    required String path,
    required Uint8List body,
    required String idemKey,
    required DateTime createdAt,
    this.status = const Value.absent(),
    this.lastAttemptAt = const Value.absent(),
    this.attemptCount = const Value.absent(),
    this.lastError = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       method = Value(method),
       path = Value(path),
       body = Value(body),
       idemKey = Value(idemKey),
       createdAt = Value(createdAt);
  static Insertable<PendingWrite> custom({
    Expression<String>? id,
    Expression<String>? method,
    Expression<String>? path,
    Expression<Uint8List>? body,
    Expression<String>? idemKey,
    Expression<DateTime>? createdAt,
    Expression<String>? status,
    Expression<DateTime>? lastAttemptAt,
    Expression<int>? attemptCount,
    Expression<String>? lastError,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (method != null) 'method': method,
      if (path != null) 'path': path,
      if (body != null) 'body': body,
      if (idemKey != null) 'idem_key': idemKey,
      if (createdAt != null) 'created_at': createdAt,
      if (status != null) 'status': status,
      if (lastAttemptAt != null) 'last_attempt_at': lastAttemptAt,
      if (attemptCount != null) 'attempt_count': attemptCount,
      if (lastError != null) 'last_error': lastError,
      if (rowid != null) 'rowid': rowid,
    });
  }

  PendingWritesCompanion copyWith({
    Value<String>? id,
    Value<String>? method,
    Value<String>? path,
    Value<Uint8List>? body,
    Value<String>? idemKey,
    Value<DateTime>? createdAt,
    Value<String>? status,
    Value<DateTime?>? lastAttemptAt,
    Value<int>? attemptCount,
    Value<String?>? lastError,
    Value<int>? rowid,
  }) {
    return PendingWritesCompanion(
      id: id ?? this.id,
      method: method ?? this.method,
      path: path ?? this.path,
      body: body ?? this.body,
      idemKey: idemKey ?? this.idemKey,
      createdAt: createdAt ?? this.createdAt,
      status: status ?? this.status,
      lastAttemptAt: lastAttemptAt ?? this.lastAttemptAt,
      attemptCount: attemptCount ?? this.attemptCount,
      lastError: lastError ?? this.lastError,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (method.present) {
      map['method'] = Variable<String>(method.value);
    }
    if (path.present) {
      map['path'] = Variable<String>(path.value);
    }
    if (body.present) {
      map['body'] = Variable<Uint8List>(body.value);
    }
    if (idemKey.present) {
      map['idem_key'] = Variable<String>(idemKey.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (status.present) {
      map['status'] = Variable<String>(status.value);
    }
    if (lastAttemptAt.present) {
      map['last_attempt_at'] = Variable<DateTime>(lastAttemptAt.value);
    }
    if (attemptCount.present) {
      map['attempt_count'] = Variable<int>(attemptCount.value);
    }
    if (lastError.present) {
      map['last_error'] = Variable<String>(lastError.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('PendingWritesCompanion(')
          ..write('id: $id, ')
          ..write('method: $method, ')
          ..write('path: $path, ')
          ..write('body: $body, ')
          ..write('idemKey: $idemKey, ')
          ..write('createdAt: $createdAt, ')
          ..write('status: $status, ')
          ..write('lastAttemptAt: $lastAttemptAt, ')
          ..write('attemptCount: $attemptCount, ')
          ..write('lastError: $lastError, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $WidgetFailuresTable extends WidgetFailures
    with TableInfo<$WidgetFailuresTable, WidgetFailure> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $WidgetFailuresTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _bodyMeta = const VerificationMeta('body');
  @override
  late final GeneratedColumn<Uint8List> body = GeneratedColumn<Uint8List>(
    'body',
    aliasedName,
    false,
    type: DriftSqlType.blob,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _idemKeyMeta = const VerificationMeta(
    'idemKey',
  );
  @override
  late final GeneratedColumn<String> idemKey = GeneratedColumn<String>(
    'idem_key',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _createdAtMeta = const VerificationMeta(
    'createdAt',
  );
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
    'created_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [id, body, idemKey, createdAt];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'widget_failures';
  @override
  VerificationContext validateIntegrity(
    Insertable<WidgetFailure> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('body')) {
      context.handle(
        _bodyMeta,
        body.isAcceptableOrUnknown(data['body']!, _bodyMeta),
      );
    } else if (isInserting) {
      context.missing(_bodyMeta);
    }
    if (data.containsKey('idem_key')) {
      context.handle(
        _idemKeyMeta,
        idemKey.isAcceptableOrUnknown(data['idem_key']!, _idemKeyMeta),
      );
    } else if (isInserting) {
      context.missing(_idemKeyMeta);
    }
    if (data.containsKey('created_at')) {
      context.handle(
        _createdAtMeta,
        createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta),
      );
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  WidgetFailure map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return WidgetFailure(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      body: attachedDatabase.typeMapping.read(
        DriftSqlType.blob,
        data['${effectivePrefix}body'],
      )!,
      idemKey: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}idem_key'],
      )!,
      createdAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}created_at'],
      )!,
    );
  }

  @override
  $WidgetFailuresTable createAlias(String alias) {
    return $WidgetFailuresTable(attachedDatabase, alias);
  }
}

class WidgetFailure extends DataClass implements Insertable<WidgetFailure> {
  final String id;
  final Uint8List body;
  final String idemKey;
  final DateTime createdAt;
  const WidgetFailure({
    required this.id,
    required this.body,
    required this.idemKey,
    required this.createdAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['body'] = Variable<Uint8List>(body);
    map['idem_key'] = Variable<String>(idemKey);
    map['created_at'] = Variable<DateTime>(createdAt);
    return map;
  }

  WidgetFailuresCompanion toCompanion(bool nullToAbsent) {
    return WidgetFailuresCompanion(
      id: Value(id),
      body: Value(body),
      idemKey: Value(idemKey),
      createdAt: Value(createdAt),
    );
  }

  factory WidgetFailure.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return WidgetFailure(
      id: serializer.fromJson<String>(json['id']),
      body: serializer.fromJson<Uint8List>(json['body']),
      idemKey: serializer.fromJson<String>(json['idemKey']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'body': serializer.toJson<Uint8List>(body),
      'idemKey': serializer.toJson<String>(idemKey),
      'createdAt': serializer.toJson<DateTime>(createdAt),
    };
  }

  WidgetFailure copyWith({
    String? id,
    Uint8List? body,
    String? idemKey,
    DateTime? createdAt,
  }) => WidgetFailure(
    id: id ?? this.id,
    body: body ?? this.body,
    idemKey: idemKey ?? this.idemKey,
    createdAt: createdAt ?? this.createdAt,
  );
  WidgetFailure copyWithCompanion(WidgetFailuresCompanion data) {
    return WidgetFailure(
      id: data.id.present ? data.id.value : this.id,
      body: data.body.present ? data.body.value : this.body,
      idemKey: data.idemKey.present ? data.idemKey.value : this.idemKey,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('WidgetFailure(')
          ..write('id: $id, ')
          ..write('body: $body, ')
          ..write('idemKey: $idemKey, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode =>
      Object.hash(id, $driftBlobEquality.hash(body), idemKey, createdAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is WidgetFailure &&
          other.id == this.id &&
          $driftBlobEquality.equals(other.body, this.body) &&
          other.idemKey == this.idemKey &&
          other.createdAt == this.createdAt);
}

class WidgetFailuresCompanion extends UpdateCompanion<WidgetFailure> {
  final Value<String> id;
  final Value<Uint8List> body;
  final Value<String> idemKey;
  final Value<DateTime> createdAt;
  final Value<int> rowid;
  const WidgetFailuresCompanion({
    this.id = const Value.absent(),
    this.body = const Value.absent(),
    this.idemKey = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  WidgetFailuresCompanion.insert({
    required String id,
    required Uint8List body,
    required String idemKey,
    required DateTime createdAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       body = Value(body),
       idemKey = Value(idemKey),
       createdAt = Value(createdAt);
  static Insertable<WidgetFailure> custom({
    Expression<String>? id,
    Expression<Uint8List>? body,
    Expression<String>? idemKey,
    Expression<DateTime>? createdAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (body != null) 'body': body,
      if (idemKey != null) 'idem_key': idemKey,
      if (createdAt != null) 'created_at': createdAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  WidgetFailuresCompanion copyWith({
    Value<String>? id,
    Value<Uint8List>? body,
    Value<String>? idemKey,
    Value<DateTime>? createdAt,
    Value<int>? rowid,
  }) {
    return WidgetFailuresCompanion(
      id: id ?? this.id,
      body: body ?? this.body,
      idemKey: idemKey ?? this.idemKey,
      createdAt: createdAt ?? this.createdAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (body.present) {
      map['body'] = Variable<Uint8List>(body.value);
    }
    if (idemKey.present) {
      map['idem_key'] = Variable<String>(idemKey.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('WidgetFailuresCompanion(')
          ..write('id: $id, ')
          ..write('body: $body, ')
          ..write('idemKey: $idemKey, ')
          ..write('createdAt: $createdAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $ChatMessagesTable extends ChatMessages
    with TableInfo<$ChatMessagesTable, ChatMessage> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $ChatMessagesTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _conversationIdMeta = const VerificationMeta(
    'conversationId',
  );
  @override
  late final GeneratedColumn<String> conversationId = GeneratedColumn<String>(
    'conversation_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _roleMeta = const VerificationMeta('role');
  @override
  late final GeneratedColumn<String> role = GeneratedColumn<String>(
    'role',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _contentMeta = const VerificationMeta(
    'content',
  );
  @override
  late final GeneratedColumn<String> content = GeneratedColumn<String>(
    'content',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _createdAtMeta = const VerificationMeta(
    'createdAt',
  );
  @override
  late final GeneratedColumn<DateTime> createdAt = GeneratedColumn<DateTime>(
    'created_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    conversationId,
    role,
    content,
    createdAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'chat_messages';
  @override
  VerificationContext validateIntegrity(
    Insertable<ChatMessage> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('conversation_id')) {
      context.handle(
        _conversationIdMeta,
        conversationId.isAcceptableOrUnknown(
          data['conversation_id']!,
          _conversationIdMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_conversationIdMeta);
    }
    if (data.containsKey('role')) {
      context.handle(
        _roleMeta,
        role.isAcceptableOrUnknown(data['role']!, _roleMeta),
      );
    } else if (isInserting) {
      context.missing(_roleMeta);
    }
    if (data.containsKey('content')) {
      context.handle(
        _contentMeta,
        content.isAcceptableOrUnknown(data['content']!, _contentMeta),
      );
    } else if (isInserting) {
      context.missing(_contentMeta);
    }
    if (data.containsKey('created_at')) {
      context.handle(
        _createdAtMeta,
        createdAt.isAcceptableOrUnknown(data['created_at']!, _createdAtMeta),
      );
    } else if (isInserting) {
      context.missing(_createdAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  ChatMessage map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return ChatMessage(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      conversationId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}conversation_id'],
      )!,
      role: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}role'],
      )!,
      content: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}content'],
      )!,
      createdAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}created_at'],
      )!,
    );
  }

  @override
  $ChatMessagesTable createAlias(String alias) {
    return $ChatMessagesTable(attachedDatabase, alias);
  }
}

class ChatMessage extends DataClass implements Insertable<ChatMessage> {
  final String id;
  final String conversationId;
  final String role;
  final String content;
  final DateTime createdAt;
  const ChatMessage({
    required this.id,
    required this.conversationId,
    required this.role,
    required this.content,
    required this.createdAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['conversation_id'] = Variable<String>(conversationId);
    map['role'] = Variable<String>(role);
    map['content'] = Variable<String>(content);
    map['created_at'] = Variable<DateTime>(createdAt);
    return map;
  }

  ChatMessagesCompanion toCompanion(bool nullToAbsent) {
    return ChatMessagesCompanion(
      id: Value(id),
      conversationId: Value(conversationId),
      role: Value(role),
      content: Value(content),
      createdAt: Value(createdAt),
    );
  }

  factory ChatMessage.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return ChatMessage(
      id: serializer.fromJson<String>(json['id']),
      conversationId: serializer.fromJson<String>(json['conversationId']),
      role: serializer.fromJson<String>(json['role']),
      content: serializer.fromJson<String>(json['content']),
      createdAt: serializer.fromJson<DateTime>(json['createdAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'conversationId': serializer.toJson<String>(conversationId),
      'role': serializer.toJson<String>(role),
      'content': serializer.toJson<String>(content),
      'createdAt': serializer.toJson<DateTime>(createdAt),
    };
  }

  ChatMessage copyWith({
    String? id,
    String? conversationId,
    String? role,
    String? content,
    DateTime? createdAt,
  }) => ChatMessage(
    id: id ?? this.id,
    conversationId: conversationId ?? this.conversationId,
    role: role ?? this.role,
    content: content ?? this.content,
    createdAt: createdAt ?? this.createdAt,
  );
  ChatMessage copyWithCompanion(ChatMessagesCompanion data) {
    return ChatMessage(
      id: data.id.present ? data.id.value : this.id,
      conversationId: data.conversationId.present
          ? data.conversationId.value
          : this.conversationId,
      role: data.role.present ? data.role.value : this.role,
      content: data.content.present ? data.content.value : this.content,
      createdAt: data.createdAt.present ? data.createdAt.value : this.createdAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('ChatMessage(')
          ..write('id: $id, ')
          ..write('conversationId: $conversationId, ')
          ..write('role: $role, ')
          ..write('content: $content, ')
          ..write('createdAt: $createdAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(id, conversationId, role, content, createdAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is ChatMessage &&
          other.id == this.id &&
          other.conversationId == this.conversationId &&
          other.role == this.role &&
          other.content == this.content &&
          other.createdAt == this.createdAt);
}

class ChatMessagesCompanion extends UpdateCompanion<ChatMessage> {
  final Value<String> id;
  final Value<String> conversationId;
  final Value<String> role;
  final Value<String> content;
  final Value<DateTime> createdAt;
  final Value<int> rowid;
  const ChatMessagesCompanion({
    this.id = const Value.absent(),
    this.conversationId = const Value.absent(),
    this.role = const Value.absent(),
    this.content = const Value.absent(),
    this.createdAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  ChatMessagesCompanion.insert({
    required String id,
    required String conversationId,
    required String role,
    required String content,
    required DateTime createdAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       conversationId = Value(conversationId),
       role = Value(role),
       content = Value(content),
       createdAt = Value(createdAt);
  static Insertable<ChatMessage> custom({
    Expression<String>? id,
    Expression<String>? conversationId,
    Expression<String>? role,
    Expression<String>? content,
    Expression<DateTime>? createdAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (conversationId != null) 'conversation_id': conversationId,
      if (role != null) 'role': role,
      if (content != null) 'content': content,
      if (createdAt != null) 'created_at': createdAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  ChatMessagesCompanion copyWith({
    Value<String>? id,
    Value<String>? conversationId,
    Value<String>? role,
    Value<String>? content,
    Value<DateTime>? createdAt,
    Value<int>? rowid,
  }) {
    return ChatMessagesCompanion(
      id: id ?? this.id,
      conversationId: conversationId ?? this.conversationId,
      role: role ?? this.role,
      content: content ?? this.content,
      createdAt: createdAt ?? this.createdAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (conversationId.present) {
      map['conversation_id'] = Variable<String>(conversationId.value);
    }
    if (role.present) {
      map['role'] = Variable<String>(role.value);
    }
    if (content.present) {
      map['content'] = Variable<String>(content.value);
    }
    if (createdAt.present) {
      map['created_at'] = Variable<DateTime>(createdAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('ChatMessagesCompanion(')
          ..write('id: $id, ')
          ..write('conversationId: $conversationId, ')
          ..write('role: $role, ')
          ..write('content: $content, ')
          ..write('createdAt: $createdAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $PlanCacheTable extends PlanCache
    with TableInfo<$PlanCacheTable, PlanCacheData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $PlanCacheTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _planDateMeta = const VerificationMeta(
    'planDate',
  );
  @override
  late final GeneratedColumn<String> planDate = GeneratedColumn<String>(
    'plan_date',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _slotMeta = const VerificationMeta('slot');
  @override
  late final GeneratedColumn<String> slot = GeneratedColumn<String>(
    'slot',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _productIdMeta = const VerificationMeta(
    'productId',
  );
  @override
  late final GeneratedColumn<String> productId = GeneratedColumn<String>(
    'product_id',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _productNameMeta = const VerificationMeta(
    'productName',
  );
  @override
  late final GeneratedColumn<String> productName = GeneratedColumn<String>(
    'product_name',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _quantityGMeta = const VerificationMeta(
    'quantityG',
  );
  @override
  late final GeneratedColumn<double> quantityG = GeneratedColumn<double>(
    'quantity_g',
    aliasedName,
    true,
    type: DriftSqlType.double,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _statusMeta = const VerificationMeta('status');
  @override
  late final GeneratedColumn<String> status = GeneratedColumn<String>(
    'status',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _refreshedAtMeta = const VerificationMeta(
    'refreshedAt',
  );
  @override
  late final GeneratedColumn<DateTime> refreshedAt = GeneratedColumn<DateTime>(
    'refreshed_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    planDate,
    slot,
    productId,
    productName,
    quantityG,
    status,
    refreshedAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'plan_cache';
  @override
  VerificationContext validateIntegrity(
    Insertable<PlanCacheData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('plan_date')) {
      context.handle(
        _planDateMeta,
        planDate.isAcceptableOrUnknown(data['plan_date']!, _planDateMeta),
      );
    } else if (isInserting) {
      context.missing(_planDateMeta);
    }
    if (data.containsKey('slot')) {
      context.handle(
        _slotMeta,
        slot.isAcceptableOrUnknown(data['slot']!, _slotMeta),
      );
    } else if (isInserting) {
      context.missing(_slotMeta);
    }
    if (data.containsKey('product_id')) {
      context.handle(
        _productIdMeta,
        productId.isAcceptableOrUnknown(data['product_id']!, _productIdMeta),
      );
    }
    if (data.containsKey('product_name')) {
      context.handle(
        _productNameMeta,
        productName.isAcceptableOrUnknown(
          data['product_name']!,
          _productNameMeta,
        ),
      );
    }
    if (data.containsKey('quantity_g')) {
      context.handle(
        _quantityGMeta,
        quantityG.isAcceptableOrUnknown(data['quantity_g']!, _quantityGMeta),
      );
    }
    if (data.containsKey('status')) {
      context.handle(
        _statusMeta,
        status.isAcceptableOrUnknown(data['status']!, _statusMeta),
      );
    } else if (isInserting) {
      context.missing(_statusMeta);
    }
    if (data.containsKey('refreshed_at')) {
      context.handle(
        _refreshedAtMeta,
        refreshedAt.isAcceptableOrUnknown(
          data['refreshed_at']!,
          _refreshedAtMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_refreshedAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  PlanCacheData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return PlanCacheData(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      planDate: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}plan_date'],
      )!,
      slot: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}slot'],
      )!,
      productId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}product_id'],
      ),
      productName: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}product_name'],
      ),
      quantityG: attachedDatabase.typeMapping.read(
        DriftSqlType.double,
        data['${effectivePrefix}quantity_g'],
      ),
      status: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}status'],
      )!,
      refreshedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}refreshed_at'],
      )!,
    );
  }

  @override
  $PlanCacheTable createAlias(String alias) {
    return $PlanCacheTable(attachedDatabase, alias);
  }
}

class PlanCacheData extends DataClass implements Insertable<PlanCacheData> {
  final String id;
  final String planDate;
  final String slot;
  final String? productId;
  final String? productName;
  final double? quantityG;
  final String status;
  final DateTime refreshedAt;
  const PlanCacheData({
    required this.id,
    required this.planDate,
    required this.slot,
    this.productId,
    this.productName,
    this.quantityG,
    required this.status,
    required this.refreshedAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['plan_date'] = Variable<String>(planDate);
    map['slot'] = Variable<String>(slot);
    if (!nullToAbsent || productId != null) {
      map['product_id'] = Variable<String>(productId);
    }
    if (!nullToAbsent || productName != null) {
      map['product_name'] = Variable<String>(productName);
    }
    if (!nullToAbsent || quantityG != null) {
      map['quantity_g'] = Variable<double>(quantityG);
    }
    map['status'] = Variable<String>(status);
    map['refreshed_at'] = Variable<DateTime>(refreshedAt);
    return map;
  }

  PlanCacheCompanion toCompanion(bool nullToAbsent) {
    return PlanCacheCompanion(
      id: Value(id),
      planDate: Value(planDate),
      slot: Value(slot),
      productId: productId == null && nullToAbsent
          ? const Value.absent()
          : Value(productId),
      productName: productName == null && nullToAbsent
          ? const Value.absent()
          : Value(productName),
      quantityG: quantityG == null && nullToAbsent
          ? const Value.absent()
          : Value(quantityG),
      status: Value(status),
      refreshedAt: Value(refreshedAt),
    );
  }

  factory PlanCacheData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return PlanCacheData(
      id: serializer.fromJson<String>(json['id']),
      planDate: serializer.fromJson<String>(json['planDate']),
      slot: serializer.fromJson<String>(json['slot']),
      productId: serializer.fromJson<String?>(json['productId']),
      productName: serializer.fromJson<String?>(json['productName']),
      quantityG: serializer.fromJson<double?>(json['quantityG']),
      status: serializer.fromJson<String>(json['status']),
      refreshedAt: serializer.fromJson<DateTime>(json['refreshedAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'planDate': serializer.toJson<String>(planDate),
      'slot': serializer.toJson<String>(slot),
      'productId': serializer.toJson<String?>(productId),
      'productName': serializer.toJson<String?>(productName),
      'quantityG': serializer.toJson<double?>(quantityG),
      'status': serializer.toJson<String>(status),
      'refreshedAt': serializer.toJson<DateTime>(refreshedAt),
    };
  }

  PlanCacheData copyWith({
    String? id,
    String? planDate,
    String? slot,
    Value<String?> productId = const Value.absent(),
    Value<String?> productName = const Value.absent(),
    Value<double?> quantityG = const Value.absent(),
    String? status,
    DateTime? refreshedAt,
  }) => PlanCacheData(
    id: id ?? this.id,
    planDate: planDate ?? this.planDate,
    slot: slot ?? this.slot,
    productId: productId.present ? productId.value : this.productId,
    productName: productName.present ? productName.value : this.productName,
    quantityG: quantityG.present ? quantityG.value : this.quantityG,
    status: status ?? this.status,
    refreshedAt: refreshedAt ?? this.refreshedAt,
  );
  PlanCacheData copyWithCompanion(PlanCacheCompanion data) {
    return PlanCacheData(
      id: data.id.present ? data.id.value : this.id,
      planDate: data.planDate.present ? data.planDate.value : this.planDate,
      slot: data.slot.present ? data.slot.value : this.slot,
      productId: data.productId.present ? data.productId.value : this.productId,
      productName: data.productName.present
          ? data.productName.value
          : this.productName,
      quantityG: data.quantityG.present ? data.quantityG.value : this.quantityG,
      status: data.status.present ? data.status.value : this.status,
      refreshedAt: data.refreshedAt.present
          ? data.refreshedAt.value
          : this.refreshedAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('PlanCacheData(')
          ..write('id: $id, ')
          ..write('planDate: $planDate, ')
          ..write('slot: $slot, ')
          ..write('productId: $productId, ')
          ..write('productName: $productName, ')
          ..write('quantityG: $quantityG, ')
          ..write('status: $status, ')
          ..write('refreshedAt: $refreshedAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    planDate,
    slot,
    productId,
    productName,
    quantityG,
    status,
    refreshedAt,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is PlanCacheData &&
          other.id == this.id &&
          other.planDate == this.planDate &&
          other.slot == this.slot &&
          other.productId == this.productId &&
          other.productName == this.productName &&
          other.quantityG == this.quantityG &&
          other.status == this.status &&
          other.refreshedAt == this.refreshedAt);
}

class PlanCacheCompanion extends UpdateCompanion<PlanCacheData> {
  final Value<String> id;
  final Value<String> planDate;
  final Value<String> slot;
  final Value<String?> productId;
  final Value<String?> productName;
  final Value<double?> quantityG;
  final Value<String> status;
  final Value<DateTime> refreshedAt;
  final Value<int> rowid;
  const PlanCacheCompanion({
    this.id = const Value.absent(),
    this.planDate = const Value.absent(),
    this.slot = const Value.absent(),
    this.productId = const Value.absent(),
    this.productName = const Value.absent(),
    this.quantityG = const Value.absent(),
    this.status = const Value.absent(),
    this.refreshedAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  PlanCacheCompanion.insert({
    required String id,
    required String planDate,
    required String slot,
    this.productId = const Value.absent(),
    this.productName = const Value.absent(),
    this.quantityG = const Value.absent(),
    required String status,
    required DateTime refreshedAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       planDate = Value(planDate),
       slot = Value(slot),
       status = Value(status),
       refreshedAt = Value(refreshedAt);
  static Insertable<PlanCacheData> custom({
    Expression<String>? id,
    Expression<String>? planDate,
    Expression<String>? slot,
    Expression<String>? productId,
    Expression<String>? productName,
    Expression<double>? quantityG,
    Expression<String>? status,
    Expression<DateTime>? refreshedAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (planDate != null) 'plan_date': planDate,
      if (slot != null) 'slot': slot,
      if (productId != null) 'product_id': productId,
      if (productName != null) 'product_name': productName,
      if (quantityG != null) 'quantity_g': quantityG,
      if (status != null) 'status': status,
      if (refreshedAt != null) 'refreshed_at': refreshedAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  PlanCacheCompanion copyWith({
    Value<String>? id,
    Value<String>? planDate,
    Value<String>? slot,
    Value<String?>? productId,
    Value<String?>? productName,
    Value<double?>? quantityG,
    Value<String>? status,
    Value<DateTime>? refreshedAt,
    Value<int>? rowid,
  }) {
    return PlanCacheCompanion(
      id: id ?? this.id,
      planDate: planDate ?? this.planDate,
      slot: slot ?? this.slot,
      productId: productId ?? this.productId,
      productName: productName ?? this.productName,
      quantityG: quantityG ?? this.quantityG,
      status: status ?? this.status,
      refreshedAt: refreshedAt ?? this.refreshedAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (planDate.present) {
      map['plan_date'] = Variable<String>(planDate.value);
    }
    if (slot.present) {
      map['slot'] = Variable<String>(slot.value);
    }
    if (productId.present) {
      map['product_id'] = Variable<String>(productId.value);
    }
    if (productName.present) {
      map['product_name'] = Variable<String>(productName.value);
    }
    if (quantityG.present) {
      map['quantity_g'] = Variable<double>(quantityG.value);
    }
    if (status.present) {
      map['status'] = Variable<String>(status.value);
    }
    if (refreshedAt.present) {
      map['refreshed_at'] = Variable<DateTime>(refreshedAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('PlanCacheCompanion(')
          ..write('id: $id, ')
          ..write('planDate: $planDate, ')
          ..write('slot: $slot, ')
          ..write('productId: $productId, ')
          ..write('productName: $productName, ')
          ..write('quantityG: $quantityG, ')
          ..write('status: $status, ')
          ..write('refreshedAt: $refreshedAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $ShoppingCacheTable extends ShoppingCache
    with TableInfo<$ShoppingCacheTable, ShoppingCacheData> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $ShoppingCacheTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _nameMeta = const VerificationMeta('name');
  @override
  late final GeneratedColumn<String> name = GeneratedColumn<String>(
    'name',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _quantityTextMeta = const VerificationMeta(
    'quantityText',
  );
  @override
  late final GeneratedColumn<String> quantityText = GeneratedColumn<String>(
    'quantity_text',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _checkedMeta = const VerificationMeta(
    'checked',
  );
  @override
  late final GeneratedColumn<bool> checked = GeneratedColumn<bool>(
    'checked',
    aliasedName,
    false,
    type: DriftSqlType.bool,
    requiredDuringInsert: false,
    defaultConstraints: GeneratedColumn.constraintIsAlways(
      'CHECK ("checked" IN (0, 1))',
    ),
    defaultValue: const Constant(false),
  );
  static const VerificationMeta _seqMeta = const VerificationMeta('seq');
  @override
  late final GeneratedColumn<int> seq = GeneratedColumn<int>(
    'seq',
    aliasedName,
    false,
    type: DriftSqlType.int,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _refreshedAtMeta = const VerificationMeta(
    'refreshedAt',
  );
  @override
  late final GeneratedColumn<DateTime> refreshedAt = GeneratedColumn<DateTime>(
    'refreshed_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    name,
    quantityText,
    checked,
    seq,
    refreshedAt,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'shopping_cache';
  @override
  VerificationContext validateIntegrity(
    Insertable<ShoppingCacheData> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('name')) {
      context.handle(
        _nameMeta,
        name.isAcceptableOrUnknown(data['name']!, _nameMeta),
      );
    } else if (isInserting) {
      context.missing(_nameMeta);
    }
    if (data.containsKey('quantity_text')) {
      context.handle(
        _quantityTextMeta,
        quantityText.isAcceptableOrUnknown(
          data['quantity_text']!,
          _quantityTextMeta,
        ),
      );
    }
    if (data.containsKey('checked')) {
      context.handle(
        _checkedMeta,
        checked.isAcceptableOrUnknown(data['checked']!, _checkedMeta),
      );
    }
    if (data.containsKey('seq')) {
      context.handle(
        _seqMeta,
        seq.isAcceptableOrUnknown(data['seq']!, _seqMeta),
      );
    } else if (isInserting) {
      context.missing(_seqMeta);
    }
    if (data.containsKey('refreshed_at')) {
      context.handle(
        _refreshedAtMeta,
        refreshedAt.isAcceptableOrUnknown(
          data['refreshed_at']!,
          _refreshedAtMeta,
        ),
      );
    } else if (isInserting) {
      context.missing(_refreshedAtMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  ShoppingCacheData map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return ShoppingCacheData(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      name: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}name'],
      )!,
      quantityText: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}quantity_text'],
      ),
      checked: attachedDatabase.typeMapping.read(
        DriftSqlType.bool,
        data['${effectivePrefix}checked'],
      )!,
      seq: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}seq'],
      )!,
      refreshedAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}refreshed_at'],
      )!,
    );
  }

  @override
  $ShoppingCacheTable createAlias(String alias) {
    return $ShoppingCacheTable(attachedDatabase, alias);
  }
}

class ShoppingCacheData extends DataClass
    implements Insertable<ShoppingCacheData> {
  final String id;
  final String name;
  final String? quantityText;
  final bool checked;
  final int seq;
  final DateTime refreshedAt;
  const ShoppingCacheData({
    required this.id,
    required this.name,
    this.quantityText,
    required this.checked,
    required this.seq,
    required this.refreshedAt,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['name'] = Variable<String>(name);
    if (!nullToAbsent || quantityText != null) {
      map['quantity_text'] = Variable<String>(quantityText);
    }
    map['checked'] = Variable<bool>(checked);
    map['seq'] = Variable<int>(seq);
    map['refreshed_at'] = Variable<DateTime>(refreshedAt);
    return map;
  }

  ShoppingCacheCompanion toCompanion(bool nullToAbsent) {
    return ShoppingCacheCompanion(
      id: Value(id),
      name: Value(name),
      quantityText: quantityText == null && nullToAbsent
          ? const Value.absent()
          : Value(quantityText),
      checked: Value(checked),
      seq: Value(seq),
      refreshedAt: Value(refreshedAt),
    );
  }

  factory ShoppingCacheData.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return ShoppingCacheData(
      id: serializer.fromJson<String>(json['id']),
      name: serializer.fromJson<String>(json['name']),
      quantityText: serializer.fromJson<String?>(json['quantityText']),
      checked: serializer.fromJson<bool>(json['checked']),
      seq: serializer.fromJson<int>(json['seq']),
      refreshedAt: serializer.fromJson<DateTime>(json['refreshedAt']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'name': serializer.toJson<String>(name),
      'quantityText': serializer.toJson<String?>(quantityText),
      'checked': serializer.toJson<bool>(checked),
      'seq': serializer.toJson<int>(seq),
      'refreshedAt': serializer.toJson<DateTime>(refreshedAt),
    };
  }

  ShoppingCacheData copyWith({
    String? id,
    String? name,
    Value<String?> quantityText = const Value.absent(),
    bool? checked,
    int? seq,
    DateTime? refreshedAt,
  }) => ShoppingCacheData(
    id: id ?? this.id,
    name: name ?? this.name,
    quantityText: quantityText.present ? quantityText.value : this.quantityText,
    checked: checked ?? this.checked,
    seq: seq ?? this.seq,
    refreshedAt: refreshedAt ?? this.refreshedAt,
  );
  ShoppingCacheData copyWithCompanion(ShoppingCacheCompanion data) {
    return ShoppingCacheData(
      id: data.id.present ? data.id.value : this.id,
      name: data.name.present ? data.name.value : this.name,
      quantityText: data.quantityText.present
          ? data.quantityText.value
          : this.quantityText,
      checked: data.checked.present ? data.checked.value : this.checked,
      seq: data.seq.present ? data.seq.value : this.seq,
      refreshedAt: data.refreshedAt.present
          ? data.refreshedAt.value
          : this.refreshedAt,
    );
  }

  @override
  String toString() {
    return (StringBuffer('ShoppingCacheData(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('quantityText: $quantityText, ')
          ..write('checked: $checked, ')
          ..write('seq: $seq, ')
          ..write('refreshedAt: $refreshedAt')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode =>
      Object.hash(id, name, quantityText, checked, seq, refreshedAt);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is ShoppingCacheData &&
          other.id == this.id &&
          other.name == this.name &&
          other.quantityText == this.quantityText &&
          other.checked == this.checked &&
          other.seq == this.seq &&
          other.refreshedAt == this.refreshedAt);
}

class ShoppingCacheCompanion extends UpdateCompanion<ShoppingCacheData> {
  final Value<String> id;
  final Value<String> name;
  final Value<String?> quantityText;
  final Value<bool> checked;
  final Value<int> seq;
  final Value<DateTime> refreshedAt;
  final Value<int> rowid;
  const ShoppingCacheCompanion({
    this.id = const Value.absent(),
    this.name = const Value.absent(),
    this.quantityText = const Value.absent(),
    this.checked = const Value.absent(),
    this.seq = const Value.absent(),
    this.refreshedAt = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  ShoppingCacheCompanion.insert({
    required String id,
    required String name,
    this.quantityText = const Value.absent(),
    this.checked = const Value.absent(),
    required int seq,
    required DateTime refreshedAt,
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       name = Value(name),
       seq = Value(seq),
       refreshedAt = Value(refreshedAt);
  static Insertable<ShoppingCacheData> custom({
    Expression<String>? id,
    Expression<String>? name,
    Expression<String>? quantityText,
    Expression<bool>? checked,
    Expression<int>? seq,
    Expression<DateTime>? refreshedAt,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (name != null) 'name': name,
      if (quantityText != null) 'quantity_text': quantityText,
      if (checked != null) 'checked': checked,
      if (seq != null) 'seq': seq,
      if (refreshedAt != null) 'refreshed_at': refreshedAt,
      if (rowid != null) 'rowid': rowid,
    });
  }

  ShoppingCacheCompanion copyWith({
    Value<String>? id,
    Value<String>? name,
    Value<String?>? quantityText,
    Value<bool>? checked,
    Value<int>? seq,
    Value<DateTime>? refreshedAt,
    Value<int>? rowid,
  }) {
    return ShoppingCacheCompanion(
      id: id ?? this.id,
      name: name ?? this.name,
      quantityText: quantityText ?? this.quantityText,
      checked: checked ?? this.checked,
      seq: seq ?? this.seq,
      refreshedAt: refreshedAt ?? this.refreshedAt,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (name.present) {
      map['name'] = Variable<String>(name.value);
    }
    if (quantityText.present) {
      map['quantity_text'] = Variable<String>(quantityText.value);
    }
    if (checked.present) {
      map['checked'] = Variable<bool>(checked.value);
    }
    if (seq.present) {
      map['seq'] = Variable<int>(seq.value);
    }
    if (refreshedAt.present) {
      map['refreshed_at'] = Variable<DateTime>(refreshedAt.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('ShoppingCacheCompanion(')
          ..write('id: $id, ')
          ..write('name: $name, ')
          ..write('quantityText: $quantityText, ')
          ..write('checked: $checked, ')
          ..write('seq: $seq, ')
          ..write('refreshedAt: $refreshedAt, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$AppDatabase extends GeneratedDatabase {
  _$AppDatabase(QueryExecutor e) : super(e);
  $AppDatabaseManager get managers => $AppDatabaseManager(this);
  late final $ProductsCacheTable productsCache = $ProductsCacheTable(this);
  late final $RecentSummaryTable recentSummary = $RecentSummaryTable(this);
  late final $PendingWritesTable pendingWrites = $PendingWritesTable(this);
  late final $WidgetFailuresTable widgetFailures = $WidgetFailuresTable(this);
  late final $ChatMessagesTable chatMessages = $ChatMessagesTable(this);
  late final $PlanCacheTable planCache = $PlanCacheTable(this);
  late final $ShoppingCacheTable shoppingCache = $ShoppingCacheTable(this);
  late final ProductsCacheDao productsCacheDao = ProductsCacheDao(
    this as AppDatabase,
  );
  late final RecentSummaryDao recentSummaryDao = RecentSummaryDao(
    this as AppDatabase,
  );
  late final PendingWritesDao pendingWritesDao = PendingWritesDao(
    this as AppDatabase,
  );
  late final WidgetFailuresDao widgetFailuresDao = WidgetFailuresDao(
    this as AppDatabase,
  );
  late final ChatMessagesDao chatMessagesDao = ChatMessagesDao(
    this as AppDatabase,
  );
  late final PlanCacheDao planCacheDao = PlanCacheDao(this as AppDatabase);
  late final ShoppingCacheDao shoppingCacheDao = ShoppingCacheDao(
    this as AppDatabase,
  );
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [
    productsCache,
    recentSummary,
    pendingWrites,
    widgetFailures,
    chatMessages,
    planCache,
    shoppingCache,
  ];
}

typedef $$ProductsCacheTableCreateCompanionBuilder =
    ProductsCacheCompanion Function({
      required String id,
      required String name,
      Value<String?> brand,
      required String source,
      required String nutrimentsPer100gJson,
      Value<double?> servingSizeG,
      Value<double?> lastLoggedQuantityG,
      Value<DateTime?> lastLoggedAt,
      required DateTime refreshedAt,
      Value<int> rowid,
    });
typedef $$ProductsCacheTableUpdateCompanionBuilder =
    ProductsCacheCompanion Function({
      Value<String> id,
      Value<String> name,
      Value<String?> brand,
      Value<String> source,
      Value<String> nutrimentsPer100gJson,
      Value<double?> servingSizeG,
      Value<double?> lastLoggedQuantityG,
      Value<DateTime?> lastLoggedAt,
      Value<DateTime> refreshedAt,
      Value<int> rowid,
    });

class $$ProductsCacheTableFilterComposer
    extends Composer<_$AppDatabase, $ProductsCacheTable> {
  $$ProductsCacheTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get brand => $composableBuilder(
    column: $table.brand,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get source => $composableBuilder(
    column: $table.source,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get nutrimentsPer100gJson => $composableBuilder(
    column: $table.nutrimentsPer100gJson,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<double> get servingSizeG => $composableBuilder(
    column: $table.servingSizeG,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<double> get lastLoggedQuantityG => $composableBuilder(
    column: $table.lastLoggedQuantityG,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get lastLoggedAt => $composableBuilder(
    column: $table.lastLoggedAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$ProductsCacheTableOrderingComposer
    extends Composer<_$AppDatabase, $ProductsCacheTable> {
  $$ProductsCacheTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get brand => $composableBuilder(
    column: $table.brand,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get source => $composableBuilder(
    column: $table.source,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get nutrimentsPer100gJson => $composableBuilder(
    column: $table.nutrimentsPer100gJson,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<double> get servingSizeG => $composableBuilder(
    column: $table.servingSizeG,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<double> get lastLoggedQuantityG => $composableBuilder(
    column: $table.lastLoggedQuantityG,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get lastLoggedAt => $composableBuilder(
    column: $table.lastLoggedAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$ProductsCacheTableAnnotationComposer
    extends Composer<_$AppDatabase, $ProductsCacheTable> {
  $$ProductsCacheTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<String> get brand =>
      $composableBuilder(column: $table.brand, builder: (column) => column);

  GeneratedColumn<String> get source =>
      $composableBuilder(column: $table.source, builder: (column) => column);

  GeneratedColumn<String> get nutrimentsPer100gJson => $composableBuilder(
    column: $table.nutrimentsPer100gJson,
    builder: (column) => column,
  );

  GeneratedColumn<double> get servingSizeG => $composableBuilder(
    column: $table.servingSizeG,
    builder: (column) => column,
  );

  GeneratedColumn<double> get lastLoggedQuantityG => $composableBuilder(
    column: $table.lastLoggedQuantityG,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get lastLoggedAt => $composableBuilder(
    column: $table.lastLoggedAt,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => column,
  );
}

class $$ProductsCacheTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $ProductsCacheTable,
          ProductsCacheData,
          $$ProductsCacheTableFilterComposer,
          $$ProductsCacheTableOrderingComposer,
          $$ProductsCacheTableAnnotationComposer,
          $$ProductsCacheTableCreateCompanionBuilder,
          $$ProductsCacheTableUpdateCompanionBuilder,
          (
            ProductsCacheData,
            BaseReferences<
              _$AppDatabase,
              $ProductsCacheTable,
              ProductsCacheData
            >,
          ),
          ProductsCacheData,
          PrefetchHooks Function()
        > {
  $$ProductsCacheTableTableManager(_$AppDatabase db, $ProductsCacheTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$ProductsCacheTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$ProductsCacheTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$ProductsCacheTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> name = const Value.absent(),
                Value<String?> brand = const Value.absent(),
                Value<String> source = const Value.absent(),
                Value<String> nutrimentsPer100gJson = const Value.absent(),
                Value<double?> servingSizeG = const Value.absent(),
                Value<double?> lastLoggedQuantityG = const Value.absent(),
                Value<DateTime?> lastLoggedAt = const Value.absent(),
                Value<DateTime> refreshedAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => ProductsCacheCompanion(
                id: id,
                name: name,
                brand: brand,
                source: source,
                nutrimentsPer100gJson: nutrimentsPer100gJson,
                servingSizeG: servingSizeG,
                lastLoggedQuantityG: lastLoggedQuantityG,
                lastLoggedAt: lastLoggedAt,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String name,
                Value<String?> brand = const Value.absent(),
                required String source,
                required String nutrimentsPer100gJson,
                Value<double?> servingSizeG = const Value.absent(),
                Value<double?> lastLoggedQuantityG = const Value.absent(),
                Value<DateTime?> lastLoggedAt = const Value.absent(),
                required DateTime refreshedAt,
                Value<int> rowid = const Value.absent(),
              }) => ProductsCacheCompanion.insert(
                id: id,
                name: name,
                brand: brand,
                source: source,
                nutrimentsPer100gJson: nutrimentsPer100gJson,
                servingSizeG: servingSizeG,
                lastLoggedQuantityG: lastLoggedQuantityG,
                lastLoggedAt: lastLoggedAt,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$ProductsCacheTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $ProductsCacheTable,
      ProductsCacheData,
      $$ProductsCacheTableFilterComposer,
      $$ProductsCacheTableOrderingComposer,
      $$ProductsCacheTableAnnotationComposer,
      $$ProductsCacheTableCreateCompanionBuilder,
      $$ProductsCacheTableUpdateCompanionBuilder,
      (
        ProductsCacheData,
        BaseReferences<_$AppDatabase, $ProductsCacheTable, ProductsCacheData>,
      ),
      ProductsCacheData,
      PrefetchHooks Function()
    >;
typedef $$RecentSummaryTableCreateCompanionBuilder =
    RecentSummaryCompanion Function({
      required String date,
      required String tz,
      required String totalsJson,
      required String entriesJson,
      required DateTime refreshedAt,
      Value<int> rowid,
    });
typedef $$RecentSummaryTableUpdateCompanionBuilder =
    RecentSummaryCompanion Function({
      Value<String> date,
      Value<String> tz,
      Value<String> totalsJson,
      Value<String> entriesJson,
      Value<DateTime> refreshedAt,
      Value<int> rowid,
    });

class $$RecentSummaryTableFilterComposer
    extends Composer<_$AppDatabase, $RecentSummaryTable> {
  $$RecentSummaryTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get tz => $composableBuilder(
    column: $table.tz,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get totalsJson => $composableBuilder(
    column: $table.totalsJson,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get entriesJson => $composableBuilder(
    column: $table.entriesJson,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$RecentSummaryTableOrderingComposer
    extends Composer<_$AppDatabase, $RecentSummaryTable> {
  $$RecentSummaryTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get date => $composableBuilder(
    column: $table.date,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get tz => $composableBuilder(
    column: $table.tz,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get totalsJson => $composableBuilder(
    column: $table.totalsJson,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get entriesJson => $composableBuilder(
    column: $table.entriesJson,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$RecentSummaryTableAnnotationComposer
    extends Composer<_$AppDatabase, $RecentSummaryTable> {
  $$RecentSummaryTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get date =>
      $composableBuilder(column: $table.date, builder: (column) => column);

  GeneratedColumn<String> get tz =>
      $composableBuilder(column: $table.tz, builder: (column) => column);

  GeneratedColumn<String> get totalsJson => $composableBuilder(
    column: $table.totalsJson,
    builder: (column) => column,
  );

  GeneratedColumn<String> get entriesJson => $composableBuilder(
    column: $table.entriesJson,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => column,
  );
}

class $$RecentSummaryTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $RecentSummaryTable,
          RecentSummaryData,
          $$RecentSummaryTableFilterComposer,
          $$RecentSummaryTableOrderingComposer,
          $$RecentSummaryTableAnnotationComposer,
          $$RecentSummaryTableCreateCompanionBuilder,
          $$RecentSummaryTableUpdateCompanionBuilder,
          (
            RecentSummaryData,
            BaseReferences<
              _$AppDatabase,
              $RecentSummaryTable,
              RecentSummaryData
            >,
          ),
          RecentSummaryData,
          PrefetchHooks Function()
        > {
  $$RecentSummaryTableTableManager(_$AppDatabase db, $RecentSummaryTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$RecentSummaryTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$RecentSummaryTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$RecentSummaryTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> date = const Value.absent(),
                Value<String> tz = const Value.absent(),
                Value<String> totalsJson = const Value.absent(),
                Value<String> entriesJson = const Value.absent(),
                Value<DateTime> refreshedAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => RecentSummaryCompanion(
                date: date,
                tz: tz,
                totalsJson: totalsJson,
                entriesJson: entriesJson,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String date,
                required String tz,
                required String totalsJson,
                required String entriesJson,
                required DateTime refreshedAt,
                Value<int> rowid = const Value.absent(),
              }) => RecentSummaryCompanion.insert(
                date: date,
                tz: tz,
                totalsJson: totalsJson,
                entriesJson: entriesJson,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$RecentSummaryTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $RecentSummaryTable,
      RecentSummaryData,
      $$RecentSummaryTableFilterComposer,
      $$RecentSummaryTableOrderingComposer,
      $$RecentSummaryTableAnnotationComposer,
      $$RecentSummaryTableCreateCompanionBuilder,
      $$RecentSummaryTableUpdateCompanionBuilder,
      (
        RecentSummaryData,
        BaseReferences<_$AppDatabase, $RecentSummaryTable, RecentSummaryData>,
      ),
      RecentSummaryData,
      PrefetchHooks Function()
    >;
typedef $$PendingWritesTableCreateCompanionBuilder =
    PendingWritesCompanion Function({
      required String id,
      required String method,
      required String path,
      required Uint8List body,
      required String idemKey,
      required DateTime createdAt,
      Value<String> status,
      Value<DateTime?> lastAttemptAt,
      Value<int> attemptCount,
      Value<String?> lastError,
      Value<int> rowid,
    });
typedef $$PendingWritesTableUpdateCompanionBuilder =
    PendingWritesCompanion Function({
      Value<String> id,
      Value<String> method,
      Value<String> path,
      Value<Uint8List> body,
      Value<String> idemKey,
      Value<DateTime> createdAt,
      Value<String> status,
      Value<DateTime?> lastAttemptAt,
      Value<int> attemptCount,
      Value<String?> lastError,
      Value<int> rowid,
    });

class $$PendingWritesTableFilterComposer
    extends Composer<_$AppDatabase, $PendingWritesTable> {
  $$PendingWritesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get method => $composableBuilder(
    column: $table.method,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get path => $composableBuilder(
    column: $table.path,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<Uint8List> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get idemKey => $composableBuilder(
    column: $table.idemKey,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get lastAttemptAt => $composableBuilder(
    column: $table.lastAttemptAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get attemptCount => $composableBuilder(
    column: $table.attemptCount,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get lastError => $composableBuilder(
    column: $table.lastError,
    builder: (column) => ColumnFilters(column),
  );
}

class $$PendingWritesTableOrderingComposer
    extends Composer<_$AppDatabase, $PendingWritesTable> {
  $$PendingWritesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get method => $composableBuilder(
    column: $table.method,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get path => $composableBuilder(
    column: $table.path,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<Uint8List> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get idemKey => $composableBuilder(
    column: $table.idemKey,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get lastAttemptAt => $composableBuilder(
    column: $table.lastAttemptAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get attemptCount => $composableBuilder(
    column: $table.attemptCount,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get lastError => $composableBuilder(
    column: $table.lastError,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$PendingWritesTableAnnotationComposer
    extends Composer<_$AppDatabase, $PendingWritesTable> {
  $$PendingWritesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get method =>
      $composableBuilder(column: $table.method, builder: (column) => column);

  GeneratedColumn<String> get path =>
      $composableBuilder(column: $table.path, builder: (column) => column);

  GeneratedColumn<Uint8List> get body =>
      $composableBuilder(column: $table.body, builder: (column) => column);

  GeneratedColumn<String> get idemKey =>
      $composableBuilder(column: $table.idemKey, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);

  GeneratedColumn<String> get status =>
      $composableBuilder(column: $table.status, builder: (column) => column);

  GeneratedColumn<DateTime> get lastAttemptAt => $composableBuilder(
    column: $table.lastAttemptAt,
    builder: (column) => column,
  );

  GeneratedColumn<int> get attemptCount => $composableBuilder(
    column: $table.attemptCount,
    builder: (column) => column,
  );

  GeneratedColumn<String> get lastError =>
      $composableBuilder(column: $table.lastError, builder: (column) => column);
}

class $$PendingWritesTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $PendingWritesTable,
          PendingWrite,
          $$PendingWritesTableFilterComposer,
          $$PendingWritesTableOrderingComposer,
          $$PendingWritesTableAnnotationComposer,
          $$PendingWritesTableCreateCompanionBuilder,
          $$PendingWritesTableUpdateCompanionBuilder,
          (
            PendingWrite,
            BaseReferences<_$AppDatabase, $PendingWritesTable, PendingWrite>,
          ),
          PendingWrite,
          PrefetchHooks Function()
        > {
  $$PendingWritesTableTableManager(_$AppDatabase db, $PendingWritesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$PendingWritesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$PendingWritesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$PendingWritesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> method = const Value.absent(),
                Value<String> path = const Value.absent(),
                Value<Uint8List> body = const Value.absent(),
                Value<String> idemKey = const Value.absent(),
                Value<DateTime> createdAt = const Value.absent(),
                Value<String> status = const Value.absent(),
                Value<DateTime?> lastAttemptAt = const Value.absent(),
                Value<int> attemptCount = const Value.absent(),
                Value<String?> lastError = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PendingWritesCompanion(
                id: id,
                method: method,
                path: path,
                body: body,
                idemKey: idemKey,
                createdAt: createdAt,
                status: status,
                lastAttemptAt: lastAttemptAt,
                attemptCount: attemptCount,
                lastError: lastError,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String method,
                required String path,
                required Uint8List body,
                required String idemKey,
                required DateTime createdAt,
                Value<String> status = const Value.absent(),
                Value<DateTime?> lastAttemptAt = const Value.absent(),
                Value<int> attemptCount = const Value.absent(),
                Value<String?> lastError = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PendingWritesCompanion.insert(
                id: id,
                method: method,
                path: path,
                body: body,
                idemKey: idemKey,
                createdAt: createdAt,
                status: status,
                lastAttemptAt: lastAttemptAt,
                attemptCount: attemptCount,
                lastError: lastError,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$PendingWritesTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $PendingWritesTable,
      PendingWrite,
      $$PendingWritesTableFilterComposer,
      $$PendingWritesTableOrderingComposer,
      $$PendingWritesTableAnnotationComposer,
      $$PendingWritesTableCreateCompanionBuilder,
      $$PendingWritesTableUpdateCompanionBuilder,
      (
        PendingWrite,
        BaseReferences<_$AppDatabase, $PendingWritesTable, PendingWrite>,
      ),
      PendingWrite,
      PrefetchHooks Function()
    >;
typedef $$WidgetFailuresTableCreateCompanionBuilder =
    WidgetFailuresCompanion Function({
      required String id,
      required Uint8List body,
      required String idemKey,
      required DateTime createdAt,
      Value<int> rowid,
    });
typedef $$WidgetFailuresTableUpdateCompanionBuilder =
    WidgetFailuresCompanion Function({
      Value<String> id,
      Value<Uint8List> body,
      Value<String> idemKey,
      Value<DateTime> createdAt,
      Value<int> rowid,
    });

class $$WidgetFailuresTableFilterComposer
    extends Composer<_$AppDatabase, $WidgetFailuresTable> {
  $$WidgetFailuresTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<Uint8List> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get idemKey => $composableBuilder(
    column: $table.idemKey,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$WidgetFailuresTableOrderingComposer
    extends Composer<_$AppDatabase, $WidgetFailuresTable> {
  $$WidgetFailuresTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<Uint8List> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get idemKey => $composableBuilder(
    column: $table.idemKey,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$WidgetFailuresTableAnnotationComposer
    extends Composer<_$AppDatabase, $WidgetFailuresTable> {
  $$WidgetFailuresTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<Uint8List> get body =>
      $composableBuilder(column: $table.body, builder: (column) => column);

  GeneratedColumn<String> get idemKey =>
      $composableBuilder(column: $table.idemKey, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);
}

class $$WidgetFailuresTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $WidgetFailuresTable,
          WidgetFailure,
          $$WidgetFailuresTableFilterComposer,
          $$WidgetFailuresTableOrderingComposer,
          $$WidgetFailuresTableAnnotationComposer,
          $$WidgetFailuresTableCreateCompanionBuilder,
          $$WidgetFailuresTableUpdateCompanionBuilder,
          (
            WidgetFailure,
            BaseReferences<_$AppDatabase, $WidgetFailuresTable, WidgetFailure>,
          ),
          WidgetFailure,
          PrefetchHooks Function()
        > {
  $$WidgetFailuresTableTableManager(
    _$AppDatabase db,
    $WidgetFailuresTable table,
  ) : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$WidgetFailuresTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$WidgetFailuresTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$WidgetFailuresTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<Uint8List> body = const Value.absent(),
                Value<String> idemKey = const Value.absent(),
                Value<DateTime> createdAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => WidgetFailuresCompanion(
                id: id,
                body: body,
                idemKey: idemKey,
                createdAt: createdAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required Uint8List body,
                required String idemKey,
                required DateTime createdAt,
                Value<int> rowid = const Value.absent(),
              }) => WidgetFailuresCompanion.insert(
                id: id,
                body: body,
                idemKey: idemKey,
                createdAt: createdAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$WidgetFailuresTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $WidgetFailuresTable,
      WidgetFailure,
      $$WidgetFailuresTableFilterComposer,
      $$WidgetFailuresTableOrderingComposer,
      $$WidgetFailuresTableAnnotationComposer,
      $$WidgetFailuresTableCreateCompanionBuilder,
      $$WidgetFailuresTableUpdateCompanionBuilder,
      (
        WidgetFailure,
        BaseReferences<_$AppDatabase, $WidgetFailuresTable, WidgetFailure>,
      ),
      WidgetFailure,
      PrefetchHooks Function()
    >;
typedef $$ChatMessagesTableCreateCompanionBuilder =
    ChatMessagesCompanion Function({
      required String id,
      required String conversationId,
      required String role,
      required String content,
      required DateTime createdAt,
      Value<int> rowid,
    });
typedef $$ChatMessagesTableUpdateCompanionBuilder =
    ChatMessagesCompanion Function({
      Value<String> id,
      Value<String> conversationId,
      Value<String> role,
      Value<String> content,
      Value<DateTime> createdAt,
      Value<int> rowid,
    });

class $$ChatMessagesTableFilterComposer
    extends Composer<_$AppDatabase, $ChatMessagesTable> {
  $$ChatMessagesTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get conversationId => $composableBuilder(
    column: $table.conversationId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get role => $composableBuilder(
    column: $table.role,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get content => $composableBuilder(
    column: $table.content,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$ChatMessagesTableOrderingComposer
    extends Composer<_$AppDatabase, $ChatMessagesTable> {
  $$ChatMessagesTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get conversationId => $composableBuilder(
    column: $table.conversationId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get role => $composableBuilder(
    column: $table.role,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get content => $composableBuilder(
    column: $table.content,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get createdAt => $composableBuilder(
    column: $table.createdAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$ChatMessagesTableAnnotationComposer
    extends Composer<_$AppDatabase, $ChatMessagesTable> {
  $$ChatMessagesTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get conversationId => $composableBuilder(
    column: $table.conversationId,
    builder: (column) => column,
  );

  GeneratedColumn<String> get role =>
      $composableBuilder(column: $table.role, builder: (column) => column);

  GeneratedColumn<String> get content =>
      $composableBuilder(column: $table.content, builder: (column) => column);

  GeneratedColumn<DateTime> get createdAt =>
      $composableBuilder(column: $table.createdAt, builder: (column) => column);
}

class $$ChatMessagesTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $ChatMessagesTable,
          ChatMessage,
          $$ChatMessagesTableFilterComposer,
          $$ChatMessagesTableOrderingComposer,
          $$ChatMessagesTableAnnotationComposer,
          $$ChatMessagesTableCreateCompanionBuilder,
          $$ChatMessagesTableUpdateCompanionBuilder,
          (
            ChatMessage,
            BaseReferences<_$AppDatabase, $ChatMessagesTable, ChatMessage>,
          ),
          ChatMessage,
          PrefetchHooks Function()
        > {
  $$ChatMessagesTableTableManager(_$AppDatabase db, $ChatMessagesTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$ChatMessagesTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$ChatMessagesTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$ChatMessagesTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> conversationId = const Value.absent(),
                Value<String> role = const Value.absent(),
                Value<String> content = const Value.absent(),
                Value<DateTime> createdAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => ChatMessagesCompanion(
                id: id,
                conversationId: conversationId,
                role: role,
                content: content,
                createdAt: createdAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String conversationId,
                required String role,
                required String content,
                required DateTime createdAt,
                Value<int> rowid = const Value.absent(),
              }) => ChatMessagesCompanion.insert(
                id: id,
                conversationId: conversationId,
                role: role,
                content: content,
                createdAt: createdAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$ChatMessagesTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $ChatMessagesTable,
      ChatMessage,
      $$ChatMessagesTableFilterComposer,
      $$ChatMessagesTableOrderingComposer,
      $$ChatMessagesTableAnnotationComposer,
      $$ChatMessagesTableCreateCompanionBuilder,
      $$ChatMessagesTableUpdateCompanionBuilder,
      (
        ChatMessage,
        BaseReferences<_$AppDatabase, $ChatMessagesTable, ChatMessage>,
      ),
      ChatMessage,
      PrefetchHooks Function()
    >;
typedef $$PlanCacheTableCreateCompanionBuilder =
    PlanCacheCompanion Function({
      required String id,
      required String planDate,
      required String slot,
      Value<String?> productId,
      Value<String?> productName,
      Value<double?> quantityG,
      required String status,
      required DateTime refreshedAt,
      Value<int> rowid,
    });
typedef $$PlanCacheTableUpdateCompanionBuilder =
    PlanCacheCompanion Function({
      Value<String> id,
      Value<String> planDate,
      Value<String> slot,
      Value<String?> productId,
      Value<String?> productName,
      Value<double?> quantityG,
      Value<String> status,
      Value<DateTime> refreshedAt,
      Value<int> rowid,
    });

class $$PlanCacheTableFilterComposer
    extends Composer<_$AppDatabase, $PlanCacheTable> {
  $$PlanCacheTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get planDate => $composableBuilder(
    column: $table.planDate,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get slot => $composableBuilder(
    column: $table.slot,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get productId => $composableBuilder(
    column: $table.productId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get productName => $composableBuilder(
    column: $table.productName,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<double> get quantityG => $composableBuilder(
    column: $table.quantityG,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$PlanCacheTableOrderingComposer
    extends Composer<_$AppDatabase, $PlanCacheTable> {
  $$PlanCacheTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get planDate => $composableBuilder(
    column: $table.planDate,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get slot => $composableBuilder(
    column: $table.slot,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get productId => $composableBuilder(
    column: $table.productId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get productName => $composableBuilder(
    column: $table.productName,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<double> get quantityG => $composableBuilder(
    column: $table.quantityG,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get status => $composableBuilder(
    column: $table.status,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$PlanCacheTableAnnotationComposer
    extends Composer<_$AppDatabase, $PlanCacheTable> {
  $$PlanCacheTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get planDate =>
      $composableBuilder(column: $table.planDate, builder: (column) => column);

  GeneratedColumn<String> get slot =>
      $composableBuilder(column: $table.slot, builder: (column) => column);

  GeneratedColumn<String> get productId =>
      $composableBuilder(column: $table.productId, builder: (column) => column);

  GeneratedColumn<String> get productName => $composableBuilder(
    column: $table.productName,
    builder: (column) => column,
  );

  GeneratedColumn<double> get quantityG =>
      $composableBuilder(column: $table.quantityG, builder: (column) => column);

  GeneratedColumn<String> get status =>
      $composableBuilder(column: $table.status, builder: (column) => column);

  GeneratedColumn<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => column,
  );
}

class $$PlanCacheTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $PlanCacheTable,
          PlanCacheData,
          $$PlanCacheTableFilterComposer,
          $$PlanCacheTableOrderingComposer,
          $$PlanCacheTableAnnotationComposer,
          $$PlanCacheTableCreateCompanionBuilder,
          $$PlanCacheTableUpdateCompanionBuilder,
          (
            PlanCacheData,
            BaseReferences<_$AppDatabase, $PlanCacheTable, PlanCacheData>,
          ),
          PlanCacheData,
          PrefetchHooks Function()
        > {
  $$PlanCacheTableTableManager(_$AppDatabase db, $PlanCacheTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$PlanCacheTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$PlanCacheTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$PlanCacheTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> planDate = const Value.absent(),
                Value<String> slot = const Value.absent(),
                Value<String?> productId = const Value.absent(),
                Value<String?> productName = const Value.absent(),
                Value<double?> quantityG = const Value.absent(),
                Value<String> status = const Value.absent(),
                Value<DateTime> refreshedAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => PlanCacheCompanion(
                id: id,
                planDate: planDate,
                slot: slot,
                productId: productId,
                productName: productName,
                quantityG: quantityG,
                status: status,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String planDate,
                required String slot,
                Value<String?> productId = const Value.absent(),
                Value<String?> productName = const Value.absent(),
                Value<double?> quantityG = const Value.absent(),
                required String status,
                required DateTime refreshedAt,
                Value<int> rowid = const Value.absent(),
              }) => PlanCacheCompanion.insert(
                id: id,
                planDate: planDate,
                slot: slot,
                productId: productId,
                productName: productName,
                quantityG: quantityG,
                status: status,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$PlanCacheTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $PlanCacheTable,
      PlanCacheData,
      $$PlanCacheTableFilterComposer,
      $$PlanCacheTableOrderingComposer,
      $$PlanCacheTableAnnotationComposer,
      $$PlanCacheTableCreateCompanionBuilder,
      $$PlanCacheTableUpdateCompanionBuilder,
      (
        PlanCacheData,
        BaseReferences<_$AppDatabase, $PlanCacheTable, PlanCacheData>,
      ),
      PlanCacheData,
      PrefetchHooks Function()
    >;
typedef $$ShoppingCacheTableCreateCompanionBuilder =
    ShoppingCacheCompanion Function({
      required String id,
      required String name,
      Value<String?> quantityText,
      Value<bool> checked,
      required int seq,
      required DateTime refreshedAt,
      Value<int> rowid,
    });
typedef $$ShoppingCacheTableUpdateCompanionBuilder =
    ShoppingCacheCompanion Function({
      Value<String> id,
      Value<String> name,
      Value<String?> quantityText,
      Value<bool> checked,
      Value<int> seq,
      Value<DateTime> refreshedAt,
      Value<int> rowid,
    });

class $$ShoppingCacheTableFilterComposer
    extends Composer<_$AppDatabase, $ShoppingCacheTable> {
  $$ShoppingCacheTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get quantityText => $composableBuilder(
    column: $table.quantityText,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<bool> get checked => $composableBuilder(
    column: $table.checked,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get seq => $composableBuilder(
    column: $table.seq,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnFilters(column),
  );
}

class $$ShoppingCacheTableOrderingComposer
    extends Composer<_$AppDatabase, $ShoppingCacheTable> {
  $$ShoppingCacheTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get name => $composableBuilder(
    column: $table.name,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get quantityText => $composableBuilder(
    column: $table.quantityText,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<bool> get checked => $composableBuilder(
    column: $table.checked,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get seq => $composableBuilder(
    column: $table.seq,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$ShoppingCacheTableAnnotationComposer
    extends Composer<_$AppDatabase, $ShoppingCacheTable> {
  $$ShoppingCacheTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get name =>
      $composableBuilder(column: $table.name, builder: (column) => column);

  GeneratedColumn<String> get quantityText => $composableBuilder(
    column: $table.quantityText,
    builder: (column) => column,
  );

  GeneratedColumn<bool> get checked =>
      $composableBuilder(column: $table.checked, builder: (column) => column);

  GeneratedColumn<int> get seq =>
      $composableBuilder(column: $table.seq, builder: (column) => column);

  GeneratedColumn<DateTime> get refreshedAt => $composableBuilder(
    column: $table.refreshedAt,
    builder: (column) => column,
  );
}

class $$ShoppingCacheTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $ShoppingCacheTable,
          ShoppingCacheData,
          $$ShoppingCacheTableFilterComposer,
          $$ShoppingCacheTableOrderingComposer,
          $$ShoppingCacheTableAnnotationComposer,
          $$ShoppingCacheTableCreateCompanionBuilder,
          $$ShoppingCacheTableUpdateCompanionBuilder,
          (
            ShoppingCacheData,
            BaseReferences<
              _$AppDatabase,
              $ShoppingCacheTable,
              ShoppingCacheData
            >,
          ),
          ShoppingCacheData,
          PrefetchHooks Function()
        > {
  $$ShoppingCacheTableTableManager(_$AppDatabase db, $ShoppingCacheTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$ShoppingCacheTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$ShoppingCacheTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$ShoppingCacheTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> name = const Value.absent(),
                Value<String?> quantityText = const Value.absent(),
                Value<bool> checked = const Value.absent(),
                Value<int> seq = const Value.absent(),
                Value<DateTime> refreshedAt = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => ShoppingCacheCompanion(
                id: id,
                name: name,
                quantityText: quantityText,
                checked: checked,
                seq: seq,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String name,
                Value<String?> quantityText = const Value.absent(),
                Value<bool> checked = const Value.absent(),
                required int seq,
                required DateTime refreshedAt,
                Value<int> rowid = const Value.absent(),
              }) => ShoppingCacheCompanion.insert(
                id: id,
                name: name,
                quantityText: quantityText,
                checked: checked,
                seq: seq,
                refreshedAt: refreshedAt,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$ShoppingCacheTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $ShoppingCacheTable,
      ShoppingCacheData,
      $$ShoppingCacheTableFilterComposer,
      $$ShoppingCacheTableOrderingComposer,
      $$ShoppingCacheTableAnnotationComposer,
      $$ShoppingCacheTableCreateCompanionBuilder,
      $$ShoppingCacheTableUpdateCompanionBuilder,
      (
        ShoppingCacheData,
        BaseReferences<_$AppDatabase, $ShoppingCacheTable, ShoppingCacheData>,
      ),
      ShoppingCacheData,
      PrefetchHooks Function()
    >;

class $AppDatabaseManager {
  final _$AppDatabase _db;
  $AppDatabaseManager(this._db);
  $$ProductsCacheTableTableManager get productsCache =>
      $$ProductsCacheTableTableManager(_db, _db.productsCache);
  $$RecentSummaryTableTableManager get recentSummary =>
      $$RecentSummaryTableTableManager(_db, _db.recentSummary);
  $$PendingWritesTableTableManager get pendingWrites =>
      $$PendingWritesTableTableManager(_db, _db.pendingWrites);
  $$WidgetFailuresTableTableManager get widgetFailures =>
      $$WidgetFailuresTableTableManager(_db, _db.widgetFailures);
  $$ChatMessagesTableTableManager get chatMessages =>
      $$ChatMessagesTableTableManager(_db, _db.chatMessages);
  $$PlanCacheTableTableManager get planCache =>
      $$PlanCacheTableTableManager(_db, _db.planCache);
  $$ShoppingCacheTableTableManager get shoppingCache =>
      $$ShoppingCacheTableTableManager(_db, _db.shoppingCache);
}

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
  final DateTime refreshedAt;
  const ProductsCacheData({
    required this.id,
    required this.name,
    this.brand,
    required this.source,
    required this.nutrimentsPer100gJson,
    this.servingSizeG,
    this.lastLoggedQuantityG,
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

abstract class _$AppDatabase extends GeneratedDatabase {
  _$AppDatabase(QueryExecutor e) : super(e);
  $AppDatabaseManager get managers => $AppDatabaseManager(this);
  late final $ProductsCacheTable productsCache = $ProductsCacheTable(this);
  late final $RecentSummaryTable recentSummary = $RecentSummaryTable(this);
  late final $PendingWritesTable pendingWrites = $PendingWritesTable(this);
  late final $WidgetFailuresTable widgetFailures = $WidgetFailuresTable(this);
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
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [
    productsCache,
    recentSummary,
    pendingWrites,
    widgetFailures,
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
}

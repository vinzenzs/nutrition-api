package com.corelyr.nutrition_companion.widget

import android.content.Context
import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.Room
import androidx.room.RoomDatabase

/**
 * Read-only-ish mirror of today's hydration progress that the home-screen
 * widget renders without waking Flutter. Flutter writes it after every
 * hydration log (via the widget bridge); the tap worker writes it after a
 * successful direct POST.
 */
@Entity(tableName = "hydration_snapshot")
data class HydrationSnapshot(
    @PrimaryKey @ColumnInfo(name = "date") val date: String,
    @ColumnInfo(name = "total_ml") val totalMl: Double,
    @ColumnInfo(name = "daily_goal_ml") val dailyGoalMl: Double,
    @ColumnInfo(name = "updated_at") val updatedAt: Long,
)

@Dao
interface HydrationSnapshotDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    fun upsert(snapshot: HydrationSnapshot)

    @Query("SELECT * FROM hydration_snapshot WHERE date = :date LIMIT 1")
    fun forDate(date: String): HydrationSnapshot?
}

@Database(entities = [HydrationSnapshot::class], version = 1, exportSchema = false)
abstract class SnapshotDb : RoomDatabase() {
    abstract fun snapshotDao(): HydrationSnapshotDao

    companion object {
        @Volatile
        private var instance: SnapshotDb? = null

        fun get(context: Context): SnapshotDb =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(
                    context.applicationContext,
                    SnapshotDb::class.java,
                    "hydration_snapshot.db",
                )
                    // Single-row snapshot read from the widget render path; a
                    // background dispatch would only add latency to a glance.
                    .allowMainThreadQueries()
                    .build()
                    .also { instance = it }
            }
    }
}

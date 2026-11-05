package com.kairos.edgevpn.ui

import android.animation.ValueAnimator
import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.ColorFilter
import android.graphics.DashPathEffect
import android.graphics.Paint
import android.graphics.PixelFormat
import android.graphics.RadialGradient
import android.graphics.RectF
import android.graphics.Shader
import android.graphics.Typeface
import android.graphics.drawable.Drawable
import android.util.AttributeSet
import android.view.MotionEvent
import android.view.View
import android.view.animation.AccelerateDecelerateInterpolator
import androidx.core.content.ContextCompat
import com.kairos.edgevpn.R
import kotlin.math.cos
import kotlin.math.hypot
import kotlin.math.min
import kotlin.math.sin

/**
 * The phone in the centre and the network's devices on a circle around it,
 * each joined to the phone by a line whose style says how it is reached.
 * While [setLooking] is on, only the phone is drawn, with a pulsing ring.
 */
class NetworkMapView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null,
) : View(context, attrs) {

    /** Called when a device on the map is tapped. */
    var onDeviceClick: ((DeviceRow) -> Unit)? = null

    private var devices: List<DeviceRow> = emptyList()
    private var looking = false
    private var aggregatedVisible = true
    private var pulseScale = 1f

    private val density = resources.displayMetrics.density
    private fun dp(v: Float) = v * density
    private fun sp(v: Float) = v * resources.displayMetrics.scaledDensity

    private val nodeRadius = dp(18f)
    private val phoneRadius = dp(21f)
    private val shadowExtra = dp(5f)
    private val labelGap = dp(4f)
    private val pulseRadius = dp(60f)

    private val linePaints = Badge.values().associateWith { mapLinePaint(it, density) }

    private val circlePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.WHITE }
    private val phonePaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val shadowPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        shader = RadialGradient(
            0f, 0f, nodeRadius + shadowExtra,
            intArrayOf(0x30000000, 0x30000000, 0x00000000),
            floatArrayOf(0f, (nodeRadius - dp(2f)) / (nodeRadius + shadowExtra), 1f),
            Shader.TileMode.CLAMP,
        )
    }
    private val labelPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = 0xFF374151.toInt()
        textSize = sp(11f)
        textAlign = Paint.Align.CENTER
    }
    private val youPaint = Paint(labelPaint).apply {
        color = ContextCompat.getColor(context, R.color.text_main)
        textSize = sp(12f)
        typeface = Typeface.DEFAULT_BOLD
    }
    private val morePaint = Paint(youPaint).apply {
        color = ContextCompat.getColor(context, R.color.text_muted)
        textSize = sp(13f)
    }
    /** Sits behind each label so a line passing under the name does not cut through it. */
    private val labelBackPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        color = ContextCompat.getColor(context, R.color.screen_bg)
        alpha = 220
    }
    private val labelBack = RectF()

    private val pulsePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = dp(2f)
        color = 0x99F59E0B.toInt()
        pathEffect = DashPathEffect(floatArrayOf(dp(6f), dp(5f)), 0f)
    }

    private val computerIcon = ContextCompat.getDrawable(context, R.drawable.ic_computer)!!.mutate()
    private val phoneIcon = ContextCompat.getDrawable(context, R.drawable.ic_phone)!!.mutate()
    private val iconTints = mapOf(
        Badge.DIRECT to ContextCompat.getColor(context, R.color.direct_fg),
        Badge.VIA_HELPER to ContextCompat.getColor(context, R.color.helper_fg),
        Badge.OFFLINE to ContextCompat.getColor(context, R.color.offline_fg),
    )

    private val pulse = ValueAnimator.ofFloat(0.9f, 1.1f).apply {
        duration = 750
        repeatCount = ValueAnimator.INFINITE
        repeatMode = ValueAnimator.REVERSE
        interpolator = AccelerateDecelerateInterpolator()
        addUpdateListener {
            pulseScale = it.animatedValue as Float
            invalidate()
        }
    }

    private var pressed: DeviceRow? = null

    init {
        updateDescription()
    }

    fun setDevices(rows: List<DeviceRow>) {
        if (rows == devices) return
        devices = rows
        updateDescription()
        invalidate()
    }

    fun setLooking(looking: Boolean) {
        if (looking == this.looking) return
        this.looking = looking
        updatePulse()
        invalidate()
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        updatePulse()
    }

    override fun onDetachedFromWindow() {
        pulse.cancel()
        super.onDetachedFromWindow()
    }

    override fun onVisibilityAggregated(isVisible: Boolean) {
        super.onVisibilityAggregated(isVisible)
        aggregatedVisible = isVisible
        updatePulse()
    }

    /** The ring animates only while it is on screen, so a connected phone does no animation work. */
    private fun updatePulse() {
        val run = looking && isAttachedToWindow && aggregatedVisible
        if (run && !pulse.isStarted) pulse.start()
        if (!run && pulse.isStarted) {
            pulse.cancel()
            pulseScale = 1f
        }
    }

    private fun updateDescription() {
        val reachable = devices.count { it.badge != Badge.OFFLINE }
        contentDescription = context.getString(
            R.string.map_description, reachable, devices.size - reachable,
        )
    }

    private val labelSpace get() = labelGap + labelPaint.textSize
    private val centreX get() = width / 2f
    private val centreY get() = (height - labelSpace) / 2f

    /** The circle's radius: 38% of the smaller side, shrunk if the labels would not fit. */
    private val ringRadius
        get() = min(0.38f * min(width, height), centreY - nodeRadius - dp(2f))

    private data class Placed(val slot: MapSlot, val x: Float, val y: Float)

    private fun placed(): List<Placed> {
        if (looking) return emptyList()
        val r = ringRadius
        return mapSlots(devices.size).map { s ->
            val a = Math.toRadians(s.angleDegrees.toDouble())
            Placed(s, centreX + r * cos(a).toFloat(), centreY + r * sin(a).toFloat())
        }
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val nodes = placed()

        for (n in nodes) {
            val i = n.slot.deviceIndex ?: continue
            canvas.drawLine(centreX, centreY, n.x, n.y, linePaints.getValue(devices[i].badge))
        }

        if (looking) {
            canvas.drawCircle(centreX, centreY, pulseRadius * pulseScale, pulsePaint)
        }

        for (n in nodes) {
            val i = n.slot.deviceIndex
            if (i != null) {
                drawDevice(canvas, n.x, n.y, devices[i])
            } else {
                drawMore(canvas, n.x, n.y, n.slot.moreCount ?: 0)
            }
        }

        drawPhone(canvas)
    }

    private fun drawBubble(canvas: Canvas, x: Float, y: Float, alpha: Int) {
        canvas.save()
        canvas.translate(x, y + dp(2f))
        shadowPaint.alpha = alpha
        canvas.drawCircle(0f, 0f, nodeRadius + shadowExtra, shadowPaint)
        canvas.restore()
        circlePaint.alpha = alpha
        canvas.drawCircle(x, y, nodeRadius, circlePaint)
    }

    private fun drawDevice(canvas: Canvas, x: Float, y: Float, d: DeviceRow) {
        val alpha = if (d.badge == Badge.OFFLINE) 128 else 255
        drawBubble(canvas, x, y, alpha)
        drawIcon(canvas, computerIcon, x, y, dp(20f), iconTints.getValue(d.badge), alpha)
        labelPaint.alpha = alpha
        val name = d.name ?: context.getString(R.string.unnamed_device)
        drawLabel(canvas, mapLabel(name), x, y + nodeRadius + labelSpace, labelPaint)
    }

    private fun drawMore(canvas: Canvas, x: Float, y: Float, count: Int) {
        drawBubble(canvas, x, y, 255)
        val text = context.getString(R.string.map_more_count, count)
        canvas.drawText(text, x, y - (morePaint.descent() + morePaint.ascent()) / 2, morePaint)
        labelPaint.alpha = 255
        drawLabel(canvas, context.getString(R.string.map_more), x, y + nodeRadius + labelSpace, labelPaint)
    }

    private fun drawPhone(canvas: Canvas) {
        phonePaint.color = ContextCompat.getColor(
            context, if (looking) R.color.looking_end else R.color.teal,
        )
        canvas.save()
        canvas.translate(centreX, centreY + dp(2f))
        shadowPaint.alpha = 255
        canvas.scale(phoneRadius / nodeRadius, phoneRadius / nodeRadius)
        canvas.drawCircle(0f, 0f, nodeRadius + shadowExtra, shadowPaint)
        canvas.restore()
        canvas.drawCircle(centreX, centreY, phoneRadius, phonePaint)
        drawIcon(canvas, phoneIcon, centreX, centreY, dp(22f), Color.WHITE, 255)
        drawLabel(canvas, context.getString(R.string.map_you), centreX, centreY + phoneRadius + labelSpace, youPaint)
    }

    private fun drawLabel(canvas: Canvas, text: String, x: Float, baseline: Float, paint: Paint) {
        val half = paint.measureText(text) / 2 + dp(4f)
        labelBack.set(x - half, baseline + paint.ascent() - dp(1f), x + half, baseline + paint.descent() + dp(1f))
        labelBackPaint.alpha = paint.alpha * 220 / 255
        canvas.drawRoundRect(labelBack, dp(6f), dp(6f), labelBackPaint)
        canvas.drawText(text, x, baseline, paint)
    }

    private fun drawIcon(canvas: Canvas, icon: Drawable, x: Float, y: Float, size: Float, tint: Int, alpha: Int) {
        val half = (size / 2).toInt()
        icon.setTint(tint)
        icon.alpha = alpha
        icon.setBounds(x.toInt() - half, y.toInt() - half, x.toInt() + half, y.toInt() + half)
        icon.draw(canvas)
    }

    private fun deviceAt(x: Float, y: Float): DeviceRow? {
        val reach = nodeRadius + dp(8f)
        return placed().firstOrNull { n ->
            n.slot.deviceIndex != null && hypot(x - n.x, y - n.y) <= reach
        }?.slot?.deviceIndex?.let { devices[it] }
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                pressed = deviceAt(event.x, event.y)
                return pressed != null
            }
            MotionEvent.ACTION_UP -> {
                val hit = deviceAt(event.x, event.y)
                val tapped = pressed
                pressed = null
                if (tapped != null && hit == tapped) {
                    performClick()
                    onDeviceClick?.invoke(tapped)
                }
                return tapped != null
            }
            MotionEvent.ACTION_CANCEL -> pressed = null
        }
        return pressed != null
    }

    override fun performClick(): Boolean {
        super.performClick()
        return true
    }
}

/** The line that joins the phone to a device reached in the [badge] way, also used by the legend. */
fun mapLinePaint(badge: Badge, density: Float): Paint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
    style = Paint.Style.STROKE
    strokeCap = Paint.Cap.ROUND
    when (badge) {
        Badge.DIRECT -> {
            color = 0xFF16A34A.toInt()
            strokeWidth = 4 * density
        }
        Badge.VIA_HELPER -> {
            color = 0xFFD97706.toInt()
            strokeWidth = 3 * density
            pathEffect = DashPathEffect(floatArrayOf(6 * density, 6 * density), 0f)
        }
        Badge.OFFLINE -> {
            color = 0xFFC4C8D0.toInt()
            strokeWidth = 2 * density
            // A near-zero dash with round caps draws round dots.
            pathEffect = DashPathEffect(floatArrayOf(0.01f, 6 * density), 0f)
        }
    }
}

/** A short sample of a map line, shown next to a legend word. */
class LegendLineDrawable(badge: Badge, private val density: Float) : Drawable() {
    private val paint = mapLinePaint(badge, density)

    override fun getIntrinsicWidth() = (18 * density).toInt()
    override fun getIntrinsicHeight() = (6 * density).toInt()

    override fun draw(canvas: Canvas) {
        val b = bounds
        val inset = paint.strokeWidth / 2
        val y = b.exactCenterY()
        canvas.drawLine(b.left + inset, y, b.right - inset, y, paint)
    }

    override fun setAlpha(alpha: Int) {
        paint.alpha = alpha
    }

    override fun setColorFilter(colorFilter: ColorFilter?) {
        paint.colorFilter = colorFilter
    }

    @Deprecated("Deprecated in Java")
    override fun getOpacity() = PixelFormat.TRANSLUCENT
}

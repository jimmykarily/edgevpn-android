package com.kairos.edgevpn.ui

import android.app.Activity
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Rect
import android.graphics.Typeface
import android.graphics.drawable.Drawable
import android.os.Handler
import android.os.Looper
import android.text.Spannable
import android.text.SpannableStringBuilder
import android.text.Spanned
import android.text.TextPaint
import android.text.method.BaseMovementMethod
import android.text.style.ClickableSpan
import android.text.style.ImageSpan
import android.text.style.StyleSpan
import android.text.style.TypefaceSpan
import android.view.MotionEvent
import android.view.TouchDelegate
import android.view.View
import android.view.ViewConfiguration
import android.view.ViewGroup
import android.widget.TextView
import androidx.annotation.StringRes
import androidx.core.content.ContextCompat
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.LifecycleOwner
import com.google.android.material.bottomsheet.BottomSheetBehavior
import com.google.android.material.bottomsheet.BottomSheetDialog
import com.kairos.edgevpn.R
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.roundToInt

private const val NBSP = ' '
private const val OBJECT = '￼'
private const val ICON_MIN_DP = 13
private const val TOUCH_DP = 48
private const val ICON_ALPHA = 179 // 70 % of the text colour, so the icon stays faded.
private const val REFRESH_MS = 2000L

private class HelpTexts(@StringRes val title: Int, @StringRes val plain: Int, @StringRes val tech: Int)

private fun texts(term: HelpTerm): HelpTexts = when (term) {
    HelpTerm.CONNECTED -> HelpTexts(R.string.help_connected_title, R.string.help_connected_plain, R.string.help_connected_tech)
    HelpTerm.LOOKING -> HelpTexts(R.string.help_looking_title, R.string.help_looking_plain, R.string.help_looking_tech)
    HelpTerm.OFF -> HelpTexts(R.string.help_off_title, R.string.help_off_plain, R.string.help_off_tech)
    HelpTerm.PROBLEM -> HelpTexts(R.string.help_problem_title, R.string.help_problem_plain, R.string.help_problem_tech)
    HelpTerm.THIS_PHONE -> HelpTexts(R.string.help_this_phone_title, R.string.help_this_phone_plain, R.string.help_this_phone_tech)
    HelpTerm.USING -> HelpTexts(R.string.help_using_title, R.string.help_using_plain, R.string.help_using_tech)
    HelpTerm.YOUR_DEVICES -> HelpTexts(R.string.help_your_devices_title, R.string.help_your_devices_plain, R.string.help_your_devices_tech)
    HelpTerm.DIRECT -> HelpTexts(R.string.help_direct_title, R.string.help_direct_plain, R.string.help_direct_tech)
    HelpTerm.VIA_HELPER -> HelpTexts(R.string.help_via_helper_title, R.string.help_via_helper_plain, R.string.help_via_helper_tech)
    HelpTerm.OFFLINE -> HelpTexts(R.string.help_offline_title, R.string.help_offline_plain, R.string.help_offline_tech)
    HelpTerm.SAME_NETWORK -> HelpTexts(R.string.help_same_network_title, R.string.help_same_network_plain, R.string.help_same_network_tech)
    HelpTerm.FINDING -> HelpTexts(R.string.help_finding_title, R.string.help_finding_plain, R.string.help_finding_tech)
    HelpTerm.HELPERS -> HelpTexts(R.string.help_helpers_title, R.string.help_helpers_plain, R.string.help_helpers_tech)
    HelpTerm.LAST_HEARD -> HelpTexts(R.string.help_last_heard_title, R.string.help_last_heard_plain, R.string.help_last_heard_tech)
    HelpTerm.NETWORK_KEY -> HelpTexts(R.string.help_network_key_title, R.string.help_network_key_plain, R.string.help_network_key_tech)
    HelpTerm.NAME -> HelpTexts(R.string.help_name_title, R.string.help_name_plain, R.string.help_name_tech)
    HelpTerm.ADDRESS -> HelpTexts(R.string.help_address_title, R.string.help_address_plain, R.string.help_address_tech)
    HelpTerm.DETAILED_LOGS -> HelpTexts(R.string.help_detailed_logs_title, R.string.help_detailed_logs_plain, R.string.help_detailed_logs_tech)
    HelpTerm.PHONE_ID -> HelpTexts(R.string.help_phone_id_title, R.string.help_phone_id_plain, R.string.help_phone_id_tech)
    HelpTerm.START_FRESH -> HelpTexts(R.string.help_start_fresh_title, R.string.help_start_fresh_plain, R.string.help_start_fresh_tech)
    HelpTerm.ADDRESS_IN_USE -> HelpTexts(R.string.help_address_in_use_title, R.string.help_address_in_use_plain, R.string.help_address_in_use_tech)
}

/**
 * The (?) icon inside a line of text. It is drawn centred on the middle of the capital letters
 * of its line, so it lines up with the words before it at any text size and on every API level,
 * and it reports the font's own metrics so the line keeps the height it has without the icon.
 */
private class HelpIconSpan(drawable: Drawable, val capHeight: Int) : ImageSpan(drawable, ALIGN_BASELINE) {
    override fun getSize(paint: Paint, text: CharSequence?, start: Int, end: Int, fm: Paint.FontMetricsInt?): Int {
        if (fm != null) paint.getFontMetricsInt(fm)
        return drawable.bounds.width()
    }

    override fun draw(
        canvas: Canvas, text: CharSequence?, start: Int, end: Int,
        x: Float, top: Int, y: Int, bottom: Int, paint: Paint,
    ) {
        val d = drawable
        canvas.save()
        canvas.translate(x, y - capHeight / 2f - d.bounds.height() / 2f)
        d.draw(canvas)
        canvas.restore()
    }
}

private class HelpClickSpan(val term: HelpTerm, val onClick: (HelpTerm) -> Unit) : ClickableSpan() {
    override fun onClick(widget: View) = onClick(term)

    // The icon carries its own look; no underline or link colour.
    override fun updateDrawState(ds: TextPaint) = Unit
}

/** The 48 dp square around the (?) icon of this TextView, in its own coordinates. */
private fun TextView.helpTouchRect(): Rect? {
    val l = layout ?: return null
    val s = text as? Spanned ?: return null
    val span = s.getSpans(0, s.length, HelpIconSpan::class.java).firstOrNull() ?: return null
    val at = s.getSpanStart(span)
    val line = l.getLineForOffset(at)
    val size = span.drawable.bounds.width()
    val cx = l.getPrimaryHorizontal(at) + totalPaddingLeft - scrollX + size / 2f
    val cy = l.getLineBaseline(line) + totalPaddingTop - scrollY - span.capHeight / 2f
    val half = TOUCH_DP * resources.displayMetrics.density / 2f
    return Rect((cx - half).roundToInt(), (cy - half).roundToInt(), (cx + half).roundToInt(), (cy + half).roundToInt())
}

private fun TextView.clickHelp() {
    val s = text as? Spanned ?: return
    s.getSpans(0, s.length, HelpClickSpan::class.java).firstOrNull()?.onClick(this)
}

/**
 * Taps on the icon area inside the TextView. Every other touch is left alone, so the
 * TextView stays as clickable as it was before (a badge still opens its device row).
 */
private object HelpIconMovement : BaseMovementMethod() {
    override fun onTouchEvent(widget: TextView, text: Spannable, event: MotionEvent): Boolean {
        val rect = widget.helpTouchRect() ?: return false
        val x = event.x.toInt()
        val y = event.y.toInt()
        return when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> rect.contains(x, y).also { widget.setTag(R.id.help_pressed, it) }
            MotionEvent.ACTION_MOVE -> widget.getTag(R.id.help_pressed) == true
            MotionEvent.ACTION_UP -> {
                val pressed = widget.getTag(R.id.help_pressed) == true
                widget.setTag(R.id.help_pressed, false)
                if (pressed && rect.contains(x, y)) widget.clickHelp()
                pressed
            }
            else -> {
                widget.setTag(R.id.help_pressed, false)
                false
            }
        }
    }
}

/**
 * Taps on the parts of the 48 dp icon areas that lie outside their TextViews. One parent may
 * hold several icons (the map legend), so this delegate keeps every TextView that has one.
 */
private class HelpTouchDelegate(private val parent: ViewGroup) : TouchDelegate(Rect(), parent) {
    val targets = LinkedHashSet<TextView>()
    private var active: TextView? = null
    private var downX = 0f
    private var downY = 0f

    private fun rectInParent(t: TextView): Rect? {
        if (t.parent !== parent || t.visibility != View.VISIBLE) return null
        return t.helpTouchRect()?.apply { offset(t.left, t.top) }
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        val x = event.x.toInt()
        val y = event.y.toInt()
        return when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                active = targets.firstOrNull { rectInParent(it)?.contains(x, y) == true }
                downX = event.x
                downY = event.y
                active != null
            }
            MotionEvent.ACTION_MOVE -> active != null
            MotionEvent.ACTION_UP -> {
                val t = active ?: return false
                active = null
                val slop = ViewConfiguration.get(parent.context).scaledTouchSlop
                if (abs(event.x - downX) <= slop && abs(event.y - downY) <= slop) t.clickHelp()
                true
            }
            else -> {
                active = null
                false
            }
        }
    }
}

/** The text without a (?) icon added earlier, so the icon can be added again after a change. */
private fun TextView.textWithoutHelpIcon(): CharSequence {
    val s = text as? Spanned ?: return text
    val span = s.getSpans(0, s.length, HelpIconSpan::class.java).firstOrNull() ?: return text
    return s.subSequence(0, max(0, s.getSpanStart(span) - 1))
}

/**
 * Adds a faded (?) icon after the last word of this TextView's text. The icon is about as tall as
 * a capital letter (never under 13 dp), sits on the middle of its line, and is held to the last
 * word by a non-breaking space so it never wraps onto a line of its own. Tapping within 48 dp of
 * it calls [onClick]. Call it again after every change of the text. An all-caps transformation
 * would drop the icon, so uppercase such text before calling this instead.
 */
fun TextView.appendHelpIcon(term: HelpTerm, onClick: (HelpTerm) -> Unit) {
    val base = textWithoutHelpIcon()

    val bounds = Rect()
    paint.getTextBounds("H", 0, 1, bounds)
    val capHeight = bounds.height()
    val size = max(capHeight, (ICON_MIN_DP * resources.displayMetrics.density).roundToInt())
    val icon = ContextCompat.getDrawable(context, R.drawable.ic_help)!!.mutate().apply {
        setBounds(0, 0, size, size)
        setTint(currentTextColor)
        alpha = ICON_ALPHA
    }

    val sb = SpannableStringBuilder(base).append(NBSP)
    val at = sb.length
    sb.append(OBJECT)
    sb.setSpan(HelpIconSpan(icon, capHeight), at, at + 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
    sb.setSpan(HelpClickSpan(term, onClick), at, at + 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)

    if (movementMethod !== HelpIconMovement) {
        // Setting a movement method makes a TextView clickable and focusable; keep what it was.
        val clickable = isClickable
        val longClickable = isLongClickable
        val focusable = isFocusable
        movementMethod = HelpIconMovement
        isClickable = clickable
        isLongClickable = longClickable
        isFocusable = focusable
    }
    setText(sb, TextView.BufferType.SPANNABLE)
    // The icon itself has no words, so TalkBack hears the text followed by "Help: <term>".
    contentDescription = "$base, " + context.getString(R.string.help_icon_description, context.getString(texts(term).title))

    val p = parent as? ViewGroup ?: return
    val delegate = p.touchDelegate as? HelpTouchDelegate ?: HelpTouchDelegate(p).also { p.touchDelegate = it }
    delegate.targets.add(this)
}

/**
 * Opens the help sheet for [term]: its title, the device name when the sheet is about one
 * device, the plain explanation, the technical details, and the values from [live]. Every
 * 2 seconds while the sheet is open and the activity is started, it asks [termNow] for the term
 * (a badge sheet follows its device's current badge) and [live] for the values, and redraws what
 * changed. Returns the sheet so the caller can dismiss it.
 */
fun showHelp(
    activity: Activity,
    term: HelpTerm,
    deviceName: String? = null,
    termNow: () -> HelpTerm = { term },
    live: () -> List<Pair<String, String>>,
): BottomSheetDialog {
    val sheet = BottomSheetDialog(activity)
    val view = activity.layoutInflater.inflate(R.layout.sheet_help, null)
    val title = view.findViewById<TextView>(R.id.helpTitle)
    val plain = view.findViewById<TextView>(R.id.helpPlain)
    val tech = view.findViewById<TextView>(R.id.helpTech)
    val liveHeading = view.findViewById<TextView>(R.id.helpLiveHeading)
    view.findViewById<TextView>(R.id.helpDevice).apply {
        text = deviceName
        visibility = if (deviceName != null) View.VISIBLE else View.GONE
    }
    val liveGroup = view.findViewById<View>(R.id.helpLiveGroup)
    val liveText = view.findViewById<TextView>(R.id.helpLive)

    val handler = Handler(Looper.getMainLooper())
    var shownTerm: HelpTerm? = null
    var shown: List<Pair<String, String>>? = null
    val update = object : Runnable {
        override fun run() {
            val t = termNow()
            if (t != shownTerm) {
                shownTerm = t
                val tx = texts(t)
                title.setText(tx.title)
                plain.setText(tx.plain)
                tech.setText(tx.tech)
                liveHeading.setText(
                    when (t.liveScope) {
                        LiveScope.CONNECTION -> R.string.help_live_connection
                        LiveScope.PHONE -> R.string.help_live_phone
                        LiveScope.NETWORK -> R.string.help_live_network
                    }
                )
            }
            val values = live()
            if (values != shown) {
                shown = values
                liveGroup.visibility = if (values.isEmpty()) View.GONE else View.VISIBLE
                liveText.text = liveRows(activity, values)
            }
            handler.postDelayed(this, REFRESH_MS)
        }
    }
    update.run()

    // Stop refreshing while the activity is stopped, and catch up when it starts again.
    val owner = activity as? LifecycleOwner
    val observer = LifecycleEventObserver { _, event ->
        when (event) {
            Lifecycle.Event.ON_STOP -> handler.removeCallbacks(update)
            Lifecycle.Event.ON_START -> {
                handler.removeCallbacks(update)
                update.run()
            }
            else -> Unit
        }
    }
    owner?.lifecycle?.addObserver(observer)
    sheet.setOnDismissListener {
        handler.removeCallbacks(update)
        owner?.lifecycle?.removeObserver(observer)
    }
    sheet.setContentView(view)
    sheet.showExpanded()
    return sheet
}

/**
 * Shows the sheet fully open instead of at its collapsed height, so on a small screen the lower
 * rows are not cut off. A sheet taller than the screen scrolls inside its content view.
 */
fun BottomSheetDialog.showExpanded() {
    behavior.skipCollapsed = true
    behavior.state = BottomSheetBehavior.STATE_EXPANDED
    show()
}

/** "Label: value" lines, with the value in bold, and in a fixed-width font when it is an address or an ID. */
private fun liveRows(activity: Activity, values: List<Pair<String, String>>): CharSequence {
    val sb = SpannableStringBuilder()
    values.forEachIndexed { i, (label, value) ->
        if (i > 0) sb.append('\n')
        sb.append(activity.getString(R.string.help_live_row, label, ""))
        val start = sb.length
        sb.append(value)
        sb.setSpan(StyleSpan(Typeface.BOLD), start, sb.length, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
        if (!value.contains(' ') && value.length > 12) {
            sb.setSpan(TypefaceSpan("monospace"), start, sb.length, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
        }
    }
    return sb
}

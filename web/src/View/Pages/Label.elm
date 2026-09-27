module View.Pages.Label exposing (view)

import Html exposing (Html, button, div, h2, input, span, text)
import Html.Attributes exposing (class, disabled, placeholder, value)
import Html.Events exposing (onClick, onInput)
import Model exposing (Field(..), FontInfo, LineForm, Model, Msg(..), RemoteData(..), Tab(..))
import View.Components.Form as Form
import View.Components.JobStatus as JobStatus
import View.Components.Preview as Preview


{-| Cap the number of fonts in the picker to keep it usable.
-}
maxFonts : Int
maxFonts =
    300


{-| The text, preview and Burn button come first so they fit on a phone
screen; the settings follow below.
-}
view : Model -> Html Msg
view model =
    let
        l =
            model.label

        s =
            model.laser

        done =
            Maybe.map .labelsDone model.machine |> Maybe.withDefault 0
    in
    div []
        [ div [ class "card label-top" ]
            [ div [] (List.indexedMap (lineEditor (fittedScale model)) l.lines)
            , div [ class "line-tools" ]
                [ button [ class "btn tiny", onClick AddLine ] [ text "+ line" ]
                , Form.checkbox "Fit to margin" l.fit ToggleFit
                ]
            , Preview.compact model model.labelPreview
            , div [ class "burn-row" ]
                [ button [ class "btn burn-btn", disabled (not (JobStatus.canRun model.machine)), onClick (RunJob LabelTab) ] [ text "🔥 Burn label" ]
                , div [ class "counter-box" ]
                    [ span [ class "counter" ] [ text (String.fromInt done) ]
                    , button [ class "btn tiny", disabled (done == 0), onClick ResetCounter ] [ text "reset" ]
                    ]
                ]
            , JobStatus.view model.machine
            ]
        , div [ class "card" ]
            [ h2 [] [ text "Font and layout" ]
            , fontPicker model
            , div [ class "fields" ]
                [ Form.select "Align" l.align [ ( "left", "Left" ), ( "center", "Center" ), ( "right", "Right" ) ] (SetField LabelAlign)
                , Form.field "Line spacing" l.spacing (SetField LabelSpacing)
                ]
            ]
        , div [ class "card" ]
            [ h2 [] [ text "Laser" ]
            , div [ class "fields" ]
                [ Form.field "Power (%)" s.power (SetField LaserPower)
                , Form.field "Feed (mm/min)" s.feed (SetField LaserFeed)
                , Form.field "Passes" s.passes (SetField LaserPasses)
                , Form.select "Mode" s.mode [ ( "fill", "Fill" ), ( "outline", "Outline" ), ( "both", "Fill + outline" ) ] (SetField LaserMode)
                , Form.field "Line interval (mm)" s.interval (SetField LaserInterval)
                , Form.field "Hatch angle (°)" s.angle (SetField LaserAngle)
                ]
            , Form.checkbox "Bidirectional hatching" s.bidir ToggleBidir
            , Form.framingSelect model.framing
            ]
        , div [ class "card" ]
            [ h2 [] [ text "Workpiece" ]
            , div [ class "fields" ]
                [ Form.field "Width (mm)" l.width (SetField LabelWidth)
                , Form.field "Height (mm)" l.height (SetField LabelHeight)
                , Form.field "Margin (mm)" l.margin (SetField LabelMargin)
                ]
            ]
        , div [ class "card" ]
            [ h2 [] [ text "Preview options" ]
            , Preview.extras model model.labelPreview
            ]
        ]


{-| The scale the server applied to fit the text, when fitting is on.
-}
fittedScale : Model -> Maybe Float
fittedScale model =
    case ( model.label.fit, model.labelPreview ) of
        ( True, Success pv ) ->
            Just pv.scale

        _ ->
            Nothing


lineEditor : Maybe Float -> Int -> LineForm -> Html Msg
lineEditor scale i line =
    let
        fitted =
            case ( scale, String.toFloat line.size ) of
                ( Just sc, Just size ) ->
                    "→ " ++ String.fromFloat (toFloat (round (size * sc * 10)) / 10)

                _ ->
                    "mm"
    in
    div [ class "line-editor" ]
        [ input [ class "line-text", placeholder "text", value line.text, onInput (SetLineText i) ] []
        , input [ class "line-size", value line.size, onInput (SetLineSize i) ] []
        , span [ class "unit" ] [ text fitted ]
        , button [ class "btn btn-small", onClick (RemoveLine i) ] [ text "✕" ]
        ]


fontPicker : Model -> Html Msg
fontPicker model =
    case model.fonts of
        Success fonts ->
            let
                needle =
                    String.toLower model.label.fontFilter

                matches =
                    fonts
                        |> List.filter (\f -> String.contains needle (String.toLower (fontName f)) || f.id == model.label.fontID)
                        |> List.take maxFonts
            in
            div [ class "fields" ]
                [ Form.field "Font filter" model.label.fontFilter (SetField LabelFontFilter)
                , Form.select "Font" model.label.fontID (List.map (\f -> ( f.id, fontName f )) matches) SetFont
                ]

        Failure e ->
            div [ class "error" ] [ text ("Fonts: " ++ e) ]

        _ ->
            div [ class "muted" ] [ text "Loading fonts…" ]


fontName : FontInfo -> String
fontName f =
    f.family ++ " " ++ f.style

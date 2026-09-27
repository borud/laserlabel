module View.Components.Preview exposing (Annotation, compact, extras, view)

import Html exposing (Html, div, li, p, pre, text, ul)
import Html.Attributes exposing (class)
import Model exposing (Model, Msg(..), Preview, Rect, RemoteData(..), Seg)
import Svg exposing (Svg, svg)
import Svg.Attributes as SA
import View.Components.Form as Form


{-| A text label drawn on the preview, in work coordinates (mm).
-}
type alias Annotation =
    { x : Float
    , y : Float
    , text : String
    , anchor : String
    }


view : Model -> List Annotation -> RemoteData Preview -> Html Msg
view model notes remote =
    case remote of
        NotAsked ->
            div [ class "preview-empty" ] [ text "No preview yet" ]

        Loading ->
            div [ class "preview-empty" ] [ text "Rendering…" ]

        Failure e ->
            div [ class "preview-empty error" ] [ text e ]

        Success pv ->
            div [ class "preview" ]
                [ drawing model.showTravel notes pv
                , warnings pv
                , stats pv
                , options model pv
                ]


{-| A small preview: drawing, warnings and a one-line summary.
-}
compact : Model -> RemoteData Preview -> Html Msg
compact model remote =
    case remote of
        Success pv ->
            div [ class "preview preview-compact" ]
                [ drawing model.showTravel [] pv
                , warnings pv
                , stats pv
                ]

        _ ->
            div [ class "preview-compact" ] [ view model [] remote ]


{-| Preview options and the G-code, for showing apart from the drawing.
-}
extras : Model -> RemoteData Preview -> Html Msg
extras model remote =
    case remote of
        Success pv ->
            options model pv

        _ ->
            text ""


options : Model -> Preview -> Html Msg
options model pv =
    div []
        [ div [ class "row" ]
            [ Form.checkbox "Show travel moves" model.showTravel ToggleTravel
            , Form.checkbox "Show G-code" model.showGcode ToggleGcode
            ]
        , if model.showGcode then
            pre [ class "gcode" ] [ text pv.gcode ]

          else
            text ""
        ]


drawing : Bool -> List Annotation -> Preview -> Html Msg
drawing showTravel notes pv =
    let
        b =
            pad
                (if List.isEmpty notes then
                    3

                 else
                    12
                )
                pv.bounds

        box =
            String.join " " (List.map String.fromFloat [ b.minX, -b.maxY, b.maxX - b.minX, b.maxY - b.minY ])
    in
    svg [ SA.viewBox box, SA.class "drawing" ]
        [ Svg.g [ SA.transform "scale(1,-1)" ]
            (List.filterMap identity
                [ Maybe.map (rect "workpiece") pv.workpiece
                , Maybe.map (rect "printable") pv.printable
                , if showTravel then
                    Just (segments "travel" pv.travel)

                  else
                    Nothing
                , Just (segments "burn" pv.burn)
                ]
            )
        , Svg.g [] (List.map annotation notes)
        ]


{-| Text is drawn outside the flipped group so it isn't mirrored.
-}
annotation : Annotation -> Svg Msg
annotation a =
    Svg.text_
        [ SA.class "annotation"
        , SA.x (String.fromFloat a.x)
        , SA.y (String.fromFloat -a.y)
        , SA.textAnchor a.anchor
        , SA.dominantBaseline "middle"
        ]
        [ Svg.text a.text ]


rect : String -> Rect -> Svg Msg
rect cls r =
    Svg.rect
        [ SA.class cls
        , SA.x (String.fromFloat r.minX)
        , SA.y (String.fromFloat r.minY)
        , SA.width (String.fromFloat (r.maxX - r.minX))
        , SA.height (String.fromFloat (r.maxY - r.minY))
        ]
        []


segments : String -> List Seg -> Svg Msg
segments cls segs =
    let
        d =
            segs
                |> List.map
                    (\s ->
                        "M" ++ String.fromFloat s.x1 ++ " " ++ String.fromFloat s.y1 ++ "L" ++ String.fromFloat s.x2 ++ " " ++ String.fromFloat s.y2
                    )
                |> String.concat
    in
    Svg.path [ SA.class cls, SA.d d ] []


pad : Float -> Rect -> Rect
pad d r =
    { minX = r.minX - d, minY = r.minY - d, maxX = r.maxX + d, maxY = r.maxY + d }


warnings : Preview -> Html Msg
warnings pv =
    if List.isEmpty pv.warnings then
        text ""

    else
        ul [ class "warnings" ] (List.map (\w -> li [] [ text w.message ]) pv.warnings)


stats : Preview -> Html Msg
stats pv =
    p [ class "stats" ]
        [ text
            (String.join " · "
                [ "burn " ++ String.fromInt (round pv.burnMM) ++ " mm"
                , "travel " ++ String.fromInt (round pv.travelMM) ++ " mm"
                , "≈ " ++ duration pv.estimateSec
                , String.fromInt pv.lines ++ " lines"
                ]
            )
        ]


duration : Float -> String
duration sec =
    let
        s =
            round sec
    in
    String.fromInt (s // 60) ++ "m " ++ String.fromInt (modBy 60 s) ++ "s"
